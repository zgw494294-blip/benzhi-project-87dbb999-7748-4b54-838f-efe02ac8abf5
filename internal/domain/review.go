package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func (a *Application) CompleteReview(reviewerID string, decisions []SiteDecision, now time.Time) error {
	if err := a.RequireState(StatePendingReview); err != nil {
		return err
	}
	var problems []Violation
	if strings.TrimSpace(reviewerID) == "" {
		problems = append(problems, violation("reviewer_id", "required", "复核员不能为空"))
	}
	if reviewerID == a.ApplicantID || reviewerID == a.ProtectionOwnerID {
		problems = append(problems, violation("reviewer_id", "not_independent", "复核员必须独立于申请人与保护责任人"))
	}
	active := a.ActiveSites()
	activeByID := map[string]*CandidateSite{}
	for _, site := range active {
		activeByID[site.ID] = site
	}
	byID := map[string]SiteDecision{}
	for i, decision := range decisions {
		field := fmt.Sprintf("decisions[%d]", i)
		if decision.Decision != "approve" && decision.Decision != "reject" && decision.Decision != "remediate" {
			problems = append(problems, violation(field+".decision", "invalid_decision", "决定必须为 approve、reject 或 remediate"))
		}
		if _, duplicate := byID[decision.SiteID]; duplicate {
			problems = append(problems, violation(field+".site_id", "duplicate_site", "同一孔位不得重复决定"))
		}
		site, current := activeByID[decision.SiteID]
		if !current {
			problems = append(problems, violation(field+".site_id", "historical_site", "决定包含历史、已撤回或已替换孔位"))
		} else if site.Status != SitePassed {
			problems = append(problems, violation(field+".site_id", "site_not_passed", "只能复核规则检查全部通过的当前孔位"))
		}
		if (decision.Decision == "reject" || decision.Decision == "remediate") && strings.TrimSpace(decision.Reason) == "" {
			problems = append(problems, violation(field+".reason", "required", "驳回或要求整改必须填写具体理由"))
		}
		decision.Reason = strings.TrimSpace(decision.Reason)
		byID[decision.SiteID] = decision
	}
	for _, site := range active {
		if _, covered := byID[site.ID]; !covered {
			problems = append(problems, violation("decisions", "incomplete", fmt.Sprintf("缺少当前孔位 %s 的复核决定", site.ID)))
		}
	}
	if len(problems) > 0 {
		return &ValidationError{Violations: problems}
	}

	normalized := make([]SiteDecision, 0, len(decisions))
	for _, decision := range decisions {
		decision.Reason = strings.TrimSpace(decision.Reason)
		normalized = append(normalized, decision)
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].SiteID < normalized[j].SiteID })
	allApproved, rejected := true, false
	for _, decision := range normalized {
		site := activeByID[decision.SiteID]
		switch decision.Decision {
		case "approve":
			site.Status = SiteApproved
		case "reject":
			site.Status = SiteRejected
			allApproved, rejected = false, true
		case "remediate":
			site.Status = SiteFailed
			allApproved = false
		}
	}
	if !allApproved {
		for _, site := range active {
			if site.Status == SiteApproved {
				site.Status = SitePassed
			}
		}
	}
	for i := range a.ReviewRounds {
		a.ReviewRounds[i].CurrentSigningBasis = false
	}
	snapshots := make([]ReviewSiteSnapshot, 0, len(active))
	for _, site := range active {
		snapshots = append(snapshots, ReviewSiteSnapshot{SiteID: site.ID, Revision: site.Revision})
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].SiteID < snapshots[j].SiteID })
	round := ReviewRound{Round: len(a.ReviewRounds) + 1, ApplicationVersion: a.Version + 1, Sites: snapshots, ReviewerID: reviewerID, Decisions: normalized, Completed: allApproved, CurrentSigningBasis: allApproved, ReviewedAt: now.UTC()}
	a.ReviewRounds = append(a.ReviewRounds, round)
	a.Review = &IndependentReview{ReviewerID: reviewerID, Decisions: normalized, Completed: allApproved, ReviewedAt: now.UTC()}
	if allApproved {
		a.State = StatePendingReview
	} else if rejected {
		a.State = StateRejected
	} else {
		a.State = StateRemediation
	}
	a.Touch(now)
	return nil
}
