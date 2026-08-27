package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type BatchRemediationInput struct {
	ID      string
	Actions []RemediationAction
	Now     time.Time
}

func (a *Application) RemediateFailedSites(in BatchRemediationInput) error {
	if err := a.RequireState(StateRemediation); err != nil {
		return err
	}
	if strings.TrimSpace(in.ID) == "" {
		return &ValidationError{Violations: []Violation{violation("remediation_batch_id", "required", "整改批次编号不能为空")}}
	}
	if len(in.Actions) == 0 {
		return &ValidationError{Violations: []Violation{violation("actions", "required", "至少提供一项整改动作")}}
	}
	sources := map[string]bool{}
	existingIDs := map[string]bool{}
	for _, site := range a.Sites {
		existingIDs[site.ID] = true
	}
	var problems []Violation
	replacements := make([]SiteInput, 0)
	replacementActionIndexes := make([]int, 0)
	for i, action := range in.Actions {
		field := fmt.Sprintf("actions[%d]", i)
		if sources[action.SiteID] {
			problems = append(problems, violation(field+".site_id", "duplicate_source", "同一源孔位不得在一批整改中重复出现"))
		}
		sources[action.SiteID] = true
		site, found := a.SiteByID(action.SiteID)
		if !found || site.Status != SiteFailed {
			problems = append(problems, violation(field+".site_id", "not_failed", "只能整改当前检查失败孔位"))
		}
		switch action.Action {
		case "withdraw":
			if action.Replacement != nil {
				problems = append(problems, violation(field+".replacement", "not_allowed", "撤回操作不得提供 replacement"))
			}
		case "revise":
			if action.Replacement == nil {
				problems = append(problems, violation(field+".replacement", "required", "修订操作必须提供 replacement"))
			} else {
				replacements = append(replacements, *action.Replacement)
				replacementActionIndexes = append(replacementActionIndexes, i)
			}
		default:
			problems = append(problems, violation(field+".action", "invalid_action", "整改操作必须为 withdraw 或 revise"))
		}
	}
	occupied := make([]Point, 0)
	for _, site := range a.Sites {
		if site.Status != SiteWithdrawn && site.Status != SiteReplaced && !sources[site.ID] {
			occupied = append(occupied, site.Position)
		}
	}
	for _, problem := range validateSiteInputs(replacements, "replacements", existingIDs, occupied) {
		var replacementIndex int
		if _, err := fmt.Sscanf(problem.Field, "replacements[%d]", &replacementIndex); err == nil && replacementIndex >= 0 && replacementIndex < len(replacementActionIndexes) {
			problem.Field = strings.Replace(problem.Field, fmt.Sprintf("replacements[%d]", replacementIndex), fmt.Sprintf("actions[%d].replacement", replacementActionIndexes[replacementIndex]), 1)
		}
		problems = append(problems, problem)
	}
	if len(problems) > 0 {
		return &ValidationError{Violations: problems}
	}

	ruleSet := map[RuleCode]bool{}
	newSiteIDs := map[string]bool{}
	for _, action := range in.Actions {
		old, _ := a.SiteByID(action.SiteID)
		a.retireSiteResults(action.SiteID)
		if action.Action == "withdraw" {
			ruleSet[RuleSpacing], ruleSet[RuleTotal] = true, true
			old.Status = SiteWithdrawn
			continue
		}
		replacement := *action.Replacement
		for _, result := range old.CheckResults {
			if !result.Passed {
				ruleSet[result.Rule] = true
			}
		}
		if !pointsEqual(old.Position, replacement.Position) {
			ruleSet[RuleBoundary], ruleSet[RuleBuffer], ruleSet[RuleSpacing] = true, true, true
		}
		if old.EstimatedVolumeMM3 != replacement.EstimatedVolumeMM3 {
			ruleSet[RuleCoreVolume], ruleSet[RuleTotal] = true, true
		}
		old.Status = SiteReplaced
		id := strings.TrimSpace(replacement.ID)
		newSiteIDs[id] = true
		a.Sites = append(a.Sites, CandidateSite{ID: id, ApplicationID: a.ID, Revision: old.Revision + 1, ReplacesSiteID: old.ID, Position: replacement.Position, DiameterMM: replacement.DiameterMM, DepthMM: replacement.DepthMM, EstimatedVolumeMM3: replacement.EstimatedVolumeMM3, NecessityNote: strings.TrimSpace(replacement.NecessityNote), Status: SiteSubmitted})
	}
	siteSet := map[string]bool{}
	if ruleSet[RuleSpacing] || ruleSet[RuleTotal] {
		for _, site := range a.ActiveSites() {
			siteSet[site.ID] = true
		}
	} else {
		for id := range newSiteIDs {
			siteSet[id] = true
		}
	}
	siteIDs := make([]string, 0, len(siteSet))
	for id := range siteSet {
		siteIDs = append(siteIDs, id)
	}
	sort.Strings(siteIDs)
	batch := RemediationBatch{ID: in.ID, Actions: len(in.Actions), SuggestedSiteIDs: siteIDs, SuggestedRules: sortedRules(ruleSet), CompletedAt: in.Now.UTC()}
	a.RemediationBatches = append(a.RemediationBatches, batch)
	a.State = StateRemediation
	a.Touch(in.Now)
	return nil
}
