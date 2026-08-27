package domain

import (
	"fmt"
	"strings"
	"time"
)

type SiteInput struct {
	ID                                      string
	Position                                Point
	DiameterMM, DepthMM, EstimatedVolumeMM3 float64
	NecessityNote                           string
}

func (a *Application) SubmitSites(inputs []SiteInput, now time.Time) error {
	if err := a.RequireState(StateBaselineFrozen); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return &ValidationError{Violations: []Violation{violation("sites", "required", "至少提交一个候选孔位")}}
	}
	problems := validateSiteInputs(inputs, "sites", nil, nil)
	if len(problems) > 0 {
		return &ValidationError{Violations: problems}
	}
	for _, input := range inputs {
		a.Sites = append(a.Sites, CandidateSite{ID: strings.TrimSpace(input.ID), ApplicationID: a.ID, Revision: 1, Position: input.Position, DiameterMM: input.DiameterMM, DepthMM: input.DepthMM, EstimatedVolumeMM3: input.EstimatedVolumeMM3, NecessityNote: strings.TrimSpace(input.NecessityNote), Status: SiteSubmitted})
	}
	a.State = StatePlanPending
	a.Touch(now)
	return nil
}

const maxNecessityNoteRunes = 2000

func validateSiteInputs(inputs []SiteInput, prefix string, existingIDs map[string]bool, occupied []Point) []Violation {
	seenIDs := map[string]int{}
	seenPositions := append([]Point(nil), occupied...)
	var problems []Violation
	for i, input := range inputs {
		field := fmt.Sprintf("%s[%d]", prefix, i)
		id := strings.TrimSpace(input.ID)
		if id == "" {
			problems = append(problems, violation(field+".site_id", "required", "孔位编号不能为空"))
		} else {
			if first, duplicate := seenIDs[id]; duplicate {
				problems = append(problems, violation(field+".site_id", "duplicate_site_id", fmt.Sprintf("孔位编号与 %s[%d] 重复", prefix, first)))
			}
			if existingIDs != nil && existingIDs[id] {
				problems = append(problems, violation(field+".site_id", "duplicate_site_id", "孔位编号已在申请中使用"))
			}
			seenIDs[id] = i
		}
		if !IsFinitePoint(input.Position) {
			problems = append(problems, violation(field+".position", "non_finite_coordinate", "候选坐标必须为有限数值"))
		} else {
			for j, position := range seenPositions {
				if pointsEqual(input.Position, position) {
					problems = append(problems, violation(field+".position", "coincident_position", fmt.Sprintf("候选坐标与已有位置 %d 完全重合", j)))
					break
				}
			}
			seenPositions = append(seenPositions, input.Position)
		}
		if !isFinite(input.DiameterMM) || !isFinite(input.DepthMM) || input.DiameterMM <= 0 || input.DepthMM <= 0 {
			problems = append(problems, violation(field+".dimensions", "invalid_parameters", "孔径和深度必须是正有限数值"))
		}
		if !isFinite(input.EstimatedVolumeMM3) || input.EstimatedVolumeMM3 <= 0 {
			problems = append(problems, violation(field+".estimated_volume_mm3", "invalid_volume", "预计样本量必须是正有限数值"))
		}
		note := strings.TrimSpace(input.NecessityNote)
		if note == "" {
			problems = append(problems, violation(field+".necessity_note", "required", "必须说明采样必要性"))
		} else if len([]rune(note)) > maxNecessityNoteRunes {
			problems = append(problems, violation(field+".necessity_note", "too_long", "采样必要性说明不得超过 2000 个字符"))
		}
	}
	return problems
}

func (a *Application) SiteByID(id string) (*CandidateSite, bool) {
	for i := range a.Sites {
		if a.Sites[i].ID == id {
			return &a.Sites[i], true
		}
	}
	return nil, false
}

func (a *Application) ActiveSites() []*CandidateSite {
	result := make([]*CandidateSite, 0)
	for i := range a.Sites {
		s := &a.Sites[i]
		if s.Status != SiteWithdrawn && s.Status != SiteReplaced {
			result = append(result, s)
		}
	}
	return result
}

func (a *Application) WithdrawFailedSite(siteID string, now time.Time) error {
	if err := a.RequireState(StateRemediation); err != nil {
		return err
	}
	site, ok := a.SiteByID(siteID)
	if !ok {
		return &ValidationError{Violations: []Violation{violation("site_id", "not_found", "孔位不存在")}}
	}
	if site.Status != SiteFailed {
		return &ValidationError{Violations: []Violation{violation("site_id", "not_failed", "只能撤回检查失败孔位")}}
	}
	a.retireSiteResults(siteID)
	site.Status = SiteWithdrawn
	a.updatePostCheckState(now)
	return nil
}

func (a *Application) ReviseFailedSite(failedID string, input SiteInput, now time.Time) error {
	if err := a.RequireState(StateRemediation); err != nil {
		return err
	}
	old, ok := a.SiteByID(failedID)
	if !ok || old.Status != SiteFailed {
		return &ValidationError{Violations: []Violation{violation("site_id", "not_failed", "只能修订检查失败孔位")}}
	}
	existing := map[string]bool{}
	occupied := make([]Point, 0)
	for _, site := range a.Sites {
		existing[site.ID] = true
		if site.Status != SiteWithdrawn && site.Status != SiteReplaced && site.ID != failedID {
			occupied = append(occupied, site.Position)
		}
	}
	if problems := validateSiteInputs([]SiteInput{input}, "replacement", existing, occupied); len(problems) > 0 {
		return &ValidationError{Violations: problems}
	}
	a.retireSiteResults(failedID)
	old.Status = SiteReplaced
	a.Sites = append(a.Sites, CandidateSite{ID: strings.TrimSpace(input.ID), ApplicationID: a.ID, Revision: old.Revision + 1, ReplacesSiteID: old.ID, Position: input.Position, DiameterMM: input.DiameterMM, DepthMM: input.DepthMM, EstimatedVolumeMM3: input.EstimatedVolumeMM3, NecessityNote: strings.TrimSpace(input.NecessityNote), Status: SiteSubmitted})
	a.Touch(now)
	return nil
}

func (a *Application) retireSiteResults(siteID string) {
	if site, found := a.SiteByID(siteID); found {
		for i := range site.CheckResults {
			site.CheckResults[i].Current = false
		}
	}
	for i := range a.CheckBatches {
		for j := range a.CheckBatches[i].Results {
			if a.CheckBatches[i].Results[j].SiteID == siteID {
				a.CheckBatches[i].Results[j].Current = false
			}
		}
	}
}
