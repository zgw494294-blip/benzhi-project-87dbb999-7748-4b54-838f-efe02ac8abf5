package domain

import (
	"fmt"
	"sort"
	"time"
)

type ResultIDFactory func(siteID string, rule RuleCode) string

type CheckBatchInput struct {
	ID          string
	Type        string
	SiteID      string
	Rules       []RuleCode
	TriggeredBy string
	Now         time.Time
	ResultIDs   ResultIDFactory
}

func (a *Application) RunChecks(now time.Time, ids ResultIDFactory) error {
	return a.RunChecksInBatch(CheckBatchInput{ID: fmt.Sprintf("batch-%d", len(a.CheckBatches)+1), Type: "full", Now: now, ResultIDs: ids})
}

func (a *Application) RunChecksInBatch(in CheckBatchInput) error {
	if err := a.RequireState(StatePlanPending); err != nil {
		return err
	}
	if a.Baseline == nil {
		return &StateError{Current: a.State, Expected: "frozen baseline"}
	}
	return a.runCheckBatch(in, a.ActiveSites(), allRules())
}

func (a *Application) RunTargetedChecks(siteID string, rules []RuleCode, now time.Time, ids ResultIDFactory) error {
	return a.RunTargetedChecksInBatch(CheckBatchInput{ID: fmt.Sprintf("batch-%d", len(a.CheckBatches)+1), Type: "targeted", SiteID: siteID, Rules: rules, Now: now, ResultIDs: ids})
}

func (a *Application) RunTargetedChecksInBatch(in CheckBatchInput) error {
	if err := a.RequireState(StateRemediation); err != nil {
		return err
	}
	site, ok := a.SiteByID(in.SiteID)
	if !ok || site.Status != SiteSubmitted {
		return &ValidationError{Violations: []Violation{violation("site_id", "not_revisable", "定向复检要求新的修订孔位")}}
	}
	rules := in.Rules
	if len(rules) == 0 {
		rules = affectedRules(site)
	}
	if problems := validateRules(rules); len(problems) > 0 {
		return &ValidationError{Violations: problems}
	}
	bySite := map[string]map[RuleCode]bool{site.ID: {}}
	for _, rule := range rules {
		bySite[site.ID][rule] = true
		if rule == RuleSpacing || rule == RuleTotal {
			for _, active := range a.ActiveSites() {
				if bySite[active.ID] == nil {
					bySite[active.ID] = map[RuleCode]bool{}
				}
				bySite[active.ID][rule] = true
			}
		}
	}
	sites := make([]*CandidateSite, 0, len(bySite))
	for _, active := range a.ActiveSites() {
		if _, included := bySite[active.ID]; included {
			sites = append(sites, active)
		}
	}
	return a.runCheckBatchBySite(in, sites, bySite)
}

func validateRules(rules []RuleCode) []Violation {
	allowed := map[RuleCode]bool{}
	for _, rule := range allRules() {
		allowed[rule] = true
	}
	var problems []Violation
	for i, rule := range rules {
		if !allowed[rule] {
			problems = append(problems, violation(fmt.Sprintf("rules[%d]", i), "unknown_rule", "包含未知检查规则"))
		}
	}
	return problems
}

func (a *Application) runCheckBatch(in CheckBatchInput, sites []*CandidateSite, rules []RuleCode) error {
	if problems := validateRules(rules); len(problems) > 0 {
		return &ValidationError{Violations: problems}
	}
	bySite := make(map[string]map[RuleCode]bool, len(sites))
	for _, site := range sites {
		bySite[site.ID] = map[RuleCode]bool{}
		for _, rule := range rules {
			bySite[site.ID][rule] = true
		}
	}
	return a.runCheckBatchBySite(in, sites, bySite)
}

func (a *Application) runCheckBatchBySite(in CheckBatchInput, sites []*CandidateSite, bySite map[string]map[RuleCode]bool) error {
	if in.ID == "" || (in.Type != "full" && in.Type != "targeted") {
		return &ValidationError{Violations: []Violation{violation("check_batch", "invalid_batch", "检查批次编号和类型无效")}}
	}
	if in.ResultIDs == nil {
		return &ValidationError{Violations: []Violation{violation("check_batch", "result_id_factory_required", "检查结果编号生成器不可用")}}
	}
	for i := range a.CheckBatches {
		a.CheckBatches[i].Current = false
	}
	active := a.ActiveSites()
	total := 0.0
	for _, site := range active {
		total += site.EstimatedVolumeMM3
	}
	results := make([]CheckResult, 0)
	version := a.Version + 1
	for _, site := range sites {
		rules := sortedRules(bySite[site.ID])
		retained := make([]CheckResult, 0, len(site.CheckResults)+len(rules))
		for _, existing := range site.CheckResults {
			if !bySite[site.ID][existing.Rule] {
				retained = append(retained, existing)
			}
		}
		for _, rule := range rules {
			passed, message := a.evaluate(site, active, total, rule)
			result := CheckResult{ID: in.ResultIDs(site.ID, rule), SiteID: site.ID, Rule: rule, Passed: passed, Message: message, CheckedAt: in.Now.UTC(), BatchID: in.ID, ApplicationVersion: version, BaselineID: a.Baseline.ID, SiteRevision: site.Revision, Current: true}
			retained = append(retained, result)
			results = append(results, result)
		}
		site.CheckResults = retained
	}
	for i := range a.CheckBatches {
		for j := range a.CheckBatches[i].Results {
			old := &a.CheckBatches[i].Results[j]
			if rules, ok := bySite[old.SiteID]; ok && rules[old.Rule] {
				old.Current = false
			}
		}
	}
	a.refreshSiteCheckStatuses()
	siteIDs := make([]string, 0, len(sites))
	for _, site := range sites {
		siteIDs = append(siteIDs, site.ID)
	}
	sort.Strings(siteIDs)
	ruleSet := map[RuleCode]bool{}
	for _, result := range results {
		ruleSet[result.Rule] = true
	}
	batch := CheckBatch{ID: in.ID, ApplicationVersion: version, BaselineID: a.Baseline.ID, Type: in.Type, SiteIDs: siteIDs, Rules: sortedRules(ruleSet), TriggeredBy: in.TriggeredBy, CompletedAt: in.Now.UTC(), ActiveSiteCount: len(active), ActiveVolumeMM3: total, VolumeLimitDifferenceMM3: a.Baseline.MaxTotalVolumeMM3 - total, Results: results, Current: true}
	batch.Summaries = summarizeResults(batch.Rules, results)
	a.CheckBatches = append(a.CheckBatches, batch)
	a.updatePostCheckState(in.Now)
	return nil
}

func summarizeResults(rules []RuleCode, results []CheckResult) []RuleSummary {
	summaries := make([]RuleSummary, 0, len(rules))
	for _, rule := range rules {
		summary := RuleSummary{Rule: rule}
		failedSites := map[string]bool{}
		for _, result := range results {
			if result.Rule != rule {
				continue
			}
			if result.Passed {
				summary.PassedCount++
			} else {
				summary.FailedCount++
				failedSites[result.SiteID] = true
			}
		}
		summary.AffectedSiteCount = len(failedSites)
		summaries = append(summaries, summary)
	}
	return summaries
}

func sortedRules(set map[RuleCode]bool) []RuleCode {
	rules := make([]RuleCode, 0, len(set))
	for _, rule := range allRules() {
		if set[rule] {
			rules = append(rules, rule)
		}
	}
	return rules
}

func (a *Application) refreshSiteCheckStatuses() {
	for _, site := range a.ActiveSites() {
		byRule := map[RuleCode]CheckResult{}
		for _, result := range site.CheckResults {
			byRule[result.Rule] = result
		}
		if len(byRule) < len(allRules()) {
			site.Status = SiteSubmitted
			continue
		}
		passed := true
		for _, rule := range allRules() {
			if !byRule[rule].Passed {
				passed = false
			}
		}
		if passed {
			site.Status = SitePassed
		} else {
			site.Status = SiteFailed
		}
	}
}

func (a *Application) evaluate(site *CandidateSite, active []*CandidateSite, total float64, rule RuleCode) (bool, string) {
	b := a.Baseline
	switch rule {
	case RuleBoundary:
		if !PointInPolygon(site.Position, b.SegmentPolygon) {
			return false, "孔位位于登记洞段边界之外"
		}
		return true, "孔位位于洞段边界内"
	case RuleBuffer:
		if DistanceToPolygonBoundary(site.Position, b.SegmentPolygon) < b.BufferDistanceMM {
			return false, "孔位侵入洞段边界缓冲带"
		}
		for _, zone := range b.ExclusionZones {
			if PointInPolygon(site.Position, zone) || DistanceToPolygonBoundary(site.Position, zone) < b.BufferDistanceMM {
				return false, "孔位侵入禁采区或其缓冲带"
			}
		}
		return true, "孔位未侵入保护缓冲区"
	case RuleSpacing:
		for _, other := range active {
			if other.ID != site.ID && Distance(site.Position, other.Position) < b.MinimumSpacingMM {
				return false, fmt.Sprintf("与孔位 %s 的间距不足", other.ID)
			}
		}
		return true, "孔位间距符合要求"
	case RuleCoreVolume:
		if site.EstimatedVolumeMM3 > b.MaxCoreVolumeMM3 {
			return false, "预计单孔样本量超过限额"
		}
		return true, "单孔样本量符合限额"
	case RuleTotal:
		if total > b.MaxTotalVolumeMM3 {
			return false, "活动孔位累计样本量超过限额"
		}
		return true, "累计样本量符合限额"
	default:
		return false, "未知检查规则"
	}
}

func (a *Application) updatePostCheckState(now time.Time) {
	active := a.ActiveSites()
	anyFailed, incomplete := false, false
	for _, site := range active {
		if site.Status == SiteFailed {
			anyFailed = true
		}
		if site.Status == SiteSubmitted {
			incomplete = true
		}
	}
	if !anyFailed && !incomplete && len(active) > 0 {
		a.State = StatePendingReview
	} else {
		a.State = StateRemediation
	}
	a.Touch(now)
}

func allRules() []RuleCode {
	return []RuleCode{RuleBoundary, RuleBuffer, RuleSpacing, RuleCoreVolume, RuleTotal}
}

func affectedRules(_ *CandidateSite) []RuleCode { return allRules() }
