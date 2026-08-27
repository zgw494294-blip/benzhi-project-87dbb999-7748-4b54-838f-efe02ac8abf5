package domain

import (
	"fmt"
	"testing"
	"time"
)

func TestWorkflowRevisionAndPermitDigest(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	app, err := CreateApplication(NewApplication{ID: "app-1", CaveName: "测试洞", SegmentName: "北段", ResearchPurpose: "年代研究", ApplicantID: "applicant", ProtectionOwnerID: "owner", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	baseline := BaselineInput{ID: "base-1", CoordinateSystem: "local-mm", SegmentPolygon: square(0, 0, 1000, 1000), ExclusionZones: []Polygon{square(400, 400, 500, 500)}, ReferencePoints: []Point{{X: 0, Y: 0}}, BufferDistanceMM: 50, MinimumSpacingMM: 100, MaxCoreVolumeMM3: 200, MaxTotalVolumeMM3: 300, Now: now}
	if err := app.FreezeBaseline(baseline); err != nil {
		t.Fatal(err)
	}
	if err := app.SubmitSites([]SiteInput{{ID: "safe", Position: Point{X: 150, Y: 150}, DiameterMM: 5, DepthMM: 10, EstimatedVolumeMM3: 100, NecessityNote: "必要"}, {ID: "risk", Position: Point{X: 450, Y: 450}, DiameterMM: 5, DepthMM: 10, EstimatedVolumeMM3: 100, NecessityNote: "必要"}}, now); err != nil {
		t.Fatal(err)
	}
	sequence := 0
	ids := func(site string, rule RuleCode) string { sequence++; return fmt.Sprintf("result-%d", sequence) }
	if err := app.RunChecks(now, ids); err != nil {
		t.Fatal(err)
	}
	if app.State != StateRemediation {
		t.Fatalf("state=%s", app.State)
	}
	if err := app.ReviseFailedSite("risk", SiteInput{ID: "risk-r2", Position: Point{X: 700, Y: 700}, DiameterMM: 5, DepthMM: 10, EstimatedVolumeMM3: 100, NecessityNote: "避让后仍必要"}, now); err != nil {
		t.Fatal(err)
	}
	if err := app.RunTargetedChecks("risk-r2", nil, now, ids); err != nil {
		t.Fatal(err)
	}
	if app.State != StatePendingReview {
		t.Fatalf("state=%s", app.State)
	}
	decisions := []SiteDecision{{SiteID: "safe", Decision: "approve"}, {SiteID: "risk-r2", Decision: "approve"}}
	if err := app.CompleteReview("reviewer", decisions, now); err != nil {
		t.Fatal(err)
	}
	if err := app.IssuePermit("permit-1", now); err != nil {
		t.Fatal(err)
	}
	if !VerifyPermitDigest(*app.Permit) {
		t.Fatal("permit digest should verify")
	}
	app.Permit.ApprovedSiteIDs[0] = "tampered"
	if VerifyPermitDigest(*app.Permit) {
		t.Fatal("tampered permit must fail verification")
	}
}

func TestWithdrawFailedSiteAdvancesRemainingPlan(t *testing.T) {
	now := time.Now().UTC()
	app, _ := CreateApplication(NewApplication{ID: "app-2", CaveName: "洞", SegmentName: "段", ResearchPurpose: "研究", ApplicantID: "a", ProtectionOwnerID: "o", Now: now})
	_ = app.FreezeBaseline(BaselineInput{ID: "b", CoordinateSystem: "local", SegmentPolygon: square(0, 0, 1000, 1000), ReferencePoints: []Point{{}}, MinimumSpacingMM: 100, MaxCoreVolumeMM3: 200, MaxTotalVolumeMM3: 300, Now: now})
	_ = app.SubmitSites([]SiteInput{{ID: "ok", Position: Point{X: 200, Y: 200}, DiameterMM: 1, DepthMM: 1, EstimatedVolumeMM3: 10, NecessityNote: "n"}, {ID: "bad", Position: Point{X: 1200, Y: 200}, DiameterMM: 1, DepthMM: 1, EstimatedVolumeMM3: 10, NecessityNote: "n"}}, now)
	_ = app.RunChecks(now, func(site string, rule RuleCode) string { return string(rule) + site })
	if err := app.WithdrawFailedSite("bad", now); err != nil {
		t.Fatal(err)
	}
	if app.State != StatePendingReview {
		t.Fatalf("state=%s", app.State)
	}
}

func TestGeometryIncludesBoundaryAndComputesDistance(t *testing.T) {
	polygon := square(0, 0, 10, 10)
	if !PointInPolygon(Point{X: 0, Y: 5}, polygon) {
		t.Fatal("boundary point should be included")
	}
	if PointInPolygon(Point{X: 11, Y: 5}, polygon) {
		t.Fatal("outside point should be excluded")
	}
	if got := DistanceToPolygonBoundary(Point{X: 5, Y: 5}, polygon); got != 5 {
		t.Fatalf("distance=%v", got)
	}
}

func TestBaselinePreflightAggregatesTopologyAndReferenceProblems(t *testing.T) {
	now := time.Now().UTC()
	app, _ := CreateApplication(NewApplication{ID: "topology", CaveName: "洞", SegmentName: "段", ResearchPurpose: "研究", ApplicantID: "a", ProtectionOwnerID: "o", Now: now})
	err := app.FreezeBaseline(BaselineInput{
		ID: "b", CoordinateSystem: "local", SegmentPolygon: Polygon{Points: []Point{{X: 0, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}, {X: 10, Y: 0}}},
		ExclusionZones: []Polygon{square(8, 2, 12, 4)}, ReferencePoints: []Point{{X: 9, Y: 3}, {X: 9, Y: 3}}, MinimumSpacingMM: 1, MaxCoreVolumeMM3: 10, MaxTotalVolumeMM3: 5, Now: now,
	})
	validation, ok := err.(*ValidationError)
	if !ok || len(validation.Violations) < 4 {
		t.Fatalf("expected aggregated violations, got %#v", err)
	}
	if app.State != StateDraft || app.Baseline != nil {
		t.Fatal("invalid baseline must not change aggregate")
	}
}

func TestCheckBatchBatchRemediationAndReviewRounds(t *testing.T) {
	now := time.Now().UTC()
	app, _ := CreateApplication(NewApplication{ID: "trace", CaveName: "洞", SegmentName: "段", ResearchPurpose: "研究", ApplicantID: "a", ProtectionOwnerID: "o", Now: now})
	if err := app.FreezeBaseline(BaselineInput{ID: "b", CoordinateSystem: "local", SegmentPolygon: square(0, 0, 1000, 1000), ReferencePoints: []Point{{X: 10, Y: 10}}, MinimumSpacingMM: 50, MaxCoreVolumeMM3: 100, MaxTotalVolumeMM3: 200, Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := app.SubmitSites([]SiteInput{{ID: "outside", Position: Point{X: 1200, Y: 200}, DiameterMM: 10, DepthMM: 10, EstimatedVolumeMM3: 50, NecessityNote: "必要"}, {ID: "over", Position: Point{X: 500, Y: 500}, DiameterMM: 10, DepthMM: 10, EstimatedVolumeMM3: 150, NecessityNote: "必要"}}, now); err != nil {
		t.Fatal(err)
	}
	seq := 0
	ids := func(site string, rule RuleCode) string { seq++; return fmt.Sprintf("r-%d", seq) }
	if err := app.RunChecksInBatch(CheckBatchInput{ID: "batch-1", Type: "full", TriggeredBy: "o", Now: now, ResultIDs: ids}); err != nil {
		t.Fatal(err)
	}
	beforeSites := len(app.Sites)
	err := app.RemediateFailedSites(BatchRemediationInput{ID: "bad-batch", Now: now, Actions: []RemediationAction{{SiteID: "over", Action: "withdraw"}, {SiteID: "missing", Action: "withdraw"}}})
	if err == nil || len(app.Sites) != beforeSites || app.Sites[1].Status != SiteFailed {
		t.Fatal("invalid remediation batch must be atomic")
	}
	if err := app.RemediateFailedSites(BatchRemediationInput{ID: "fix-1", Now: now, Actions: []RemediationAction{{SiteID: "over", Action: "withdraw"}, {SiteID: "outside", Action: "revise", Replacement: &SiteInput{ID: "inside-r2", Position: Point{X: 300, Y: 300}, DiameterMM: 10, DepthMM: 10, EstimatedVolumeMM3: 50, NecessityNote: "调整位置"}}}}); err != nil {
		t.Fatal(err)
	}
	if len(app.RemediationBatches) != 1 || app.Sites[0].Status != SiteReplaced || app.Sites[1].Status != SiteWithdrawn {
		t.Fatal("successful batch did not preserve remediation lineage")
	}
	if err := app.RunTargetedChecksInBatch(CheckBatchInput{ID: "batch-2", Type: "targeted", SiteID: "inside-r2", TriggeredBy: "o", Now: now, ResultIDs: ids}); err != nil {
		t.Fatal(err)
	}
	if app.CheckBatches[0].Current || !app.CheckBatches[1].Current {
		t.Fatal("check batch current flags are incorrect")
	}
	if err := app.CompleteReview("reviewer", []SiteDecision{{SiteID: "inside-r2", Decision: "remediate", Reason: "需要进一步避让"}}, now); err != nil {
		t.Fatal(err)
	}
	if err := app.ReviseFailedSite("inside-r2", SiteInput{ID: "inside-r3", Position: Point{X: 700, Y: 700}, DiameterMM: 10, DepthMM: 10, EstimatedVolumeMM3: 50, NecessityNote: "再次调整"}, now); err != nil {
		t.Fatal(err)
	}
	if err := app.RunTargetedChecks("inside-r3", nil, now, ids); err != nil {
		t.Fatal(err)
	}
	if err := app.CompleteReview("reviewer", []SiteDecision{{SiteID: "inside-r3", Decision: "approve"}}, now); err != nil {
		t.Fatal(err)
	}
	if len(app.ReviewRounds) != 2 || app.ReviewRounds[0].CurrentSigningBasis || !app.ReviewRounds[1].CurrentSigningBasis {
		t.Fatal("review history or signing basis is incorrect")
	}
	if err := app.IssuePermit("permit-trace", now); err != nil {
		t.Fatal(err)
	}
	// Repository.Update 会提交 IssuePermit 预留的版本。
	app.Version++
	verification := VerifyPermit(*app.Permit, app)
	if !verification.Verified || verification.Scope == nil {
		t.Fatal("issued permit should verify")
	}
	tampered := *app.Permit
	tampered.ApprovedSites = append([]PermitSiteSnapshot(nil), app.Permit.ApprovedSites...)
	tampered.ApprovedSites[0].EstimatedVolumeMM3++
	verification = VerifyPermit(tampered, app)
	if verification.Verified || verification.Scope != nil || verification.Checks.ContentDigest {
		t.Fatal("tampered snapshot must remain identifiable but invalid")
	}
}

func square(minX, minY, maxX, maxY float64) Polygon {
	return Polygon{Points: []Point{{X: minX, Y: minY}, {X: maxX, Y: minY}, {X: maxX, Y: maxY}, {X: minX, Y: maxY}}}
}
