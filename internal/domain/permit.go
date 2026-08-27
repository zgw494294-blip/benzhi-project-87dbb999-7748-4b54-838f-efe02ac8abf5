package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

type permitDigestPayload struct {
	PermitID           string                 `json:"permit_id"`
	ApplicationID      string                 `json:"application_id"`
	ApplicationVersion int64                  `json:"application_version"`
	Baseline           PermitBaselineSnapshot `json:"baseline"`
	ApprovedSiteIDs    []string               `json:"approved_site_ids"`
	ApprovedSites      []PermitSiteSnapshot   `json:"approved_sites"`
	FinalReviewRound   int                    `json:"final_review_round"`
	ReviewerID         string                 `json:"reviewer_id"`
	IssuedAt           time.Time              `json:"issued_at"`
	Status             string                 `json:"status"`
}

type baselineDigestPayload struct {
	BaselineID        string    `json:"baseline_id"`
	CoordinateSystem  string    `json:"coordinate_system"`
	SegmentPolygon    Polygon   `json:"segment_polygon"`
	ExclusionZones    []Polygon `json:"exclusion_zones"`
	ReferencePoints   []Point   `json:"reference_points"`
	BufferDistanceMM  float64   `json:"buffer_distance_mm"`
	MinimumSpacingMM  float64   `json:"minimum_spacing_mm"`
	MaxCoreVolumeMM3  float64   `json:"max_core_volume_mm3"`
	MaxTotalVolumeMM3 float64   `json:"max_total_volume_mm3"`
}

type PermitVerificationChecks struct {
	ContentDigest      bool `json:"content_digest"`
	BaselineDigest     bool `json:"baseline_digest"`
	UniqueOwnership    bool `json:"unique_ownership"`
	ApplicationVersion bool `json:"application_version"`
	ApprovedSiteScope  bool `json:"approved_site_scope"`
	FinalReviewRound   bool `json:"final_review_round"`
}

type PermitVerification struct {
	Verified       bool                     `json:"verified"`
	Status         string                   `json:"status"`
	Permit         *SamplingPermit          `json:"permit"`
	Checks         PermitVerificationChecks `json:"checks"`
	Scope          *PermitScope             `json:"scope,omitempty"`
	ContentSummary string                   `json:"content_summary"`
}

type PermitScope struct {
	ApplicationID    string               `json:"application_id"`
	BaselineID       string               `json:"baseline_id"`
	ApprovedSites    []PermitSiteSnapshot `json:"approved_sites"`
	FinalReviewRound int                  `json:"final_review_round"`
}

func (a *Application) IssuePermit(permitID string, now time.Time) error {
	if a.Permit != nil {
		return &ValidationError{Violations: []Violation{violation("permit", "already_issued", "申请已签发许可证")}}
	}
	if err := a.RequireState(StatePendingReview); err != nil {
		return err
	}
	if a.Baseline == nil || a.Review == nil || !a.Review.Completed || len(a.ReviewRounds) == 0 {
		return &ValidationError{Violations: []Violation{violation("review", "not_completed", "最新独立复核尚未全部通过")}}
	}
	latest := a.ReviewRounds[len(a.ReviewRounds)-1]
	if !latest.Completed || !latest.CurrentSigningBasis {
		return &ValidationError{Violations: []Violation{violation("review", "not_current", "只有最新且全部通过的复核轮次可作为签发依据")}}
	}
	approved := make([]PermitSiteSnapshot, 0)
	for _, site := range a.ActiveSites() {
		if site.Status != SiteApproved {
			return &ValidationError{Violations: []Violation{violation("sites", "not_approved", "所有活动孔位必须获准")}}
		}
		approved = append(approved, PermitSiteSnapshot{SiteID: site.ID, Revision: site.Revision, Position: site.Position, DiameterMM: site.DiameterMM, DepthMM: site.DepthMM, EstimatedVolumeMM3: site.EstimatedVolumeMM3})
	}
	if permitID == "" || len(approved) == 0 {
		return &ValidationError{Violations: []Violation{violation("permit_id", "required", "许可证编号不能为空且必须包含获准孔位")}}
	}
	sort.Slice(approved, func(i, j int) bool { return approved[i].SiteID < approved[j].SiteID })
	baseline := permitBaselineSnapshot(*a.Baseline)
	issued := now.UTC()
	nextVersion := a.Version + 1
	ids := make([]string, len(approved))
	for i := range approved {
		ids[i] = approved[i].SiteID
	}
	permit := SamplingPermit{ID: permitID, ApplicationID: a.ID, ApplicationVersion: nextVersion, ApprovedSiteIDs: ids, ReviewerID: latest.ReviewerID, IssuedAt: issued, Status: "valid", Baseline: baseline, ApprovedSites: approved, FinalReviewRound: latest.Round}
	permit.ContentDigest = digestPayload(permitPayload(permit))
	a.Permit = &permit
	a.State = StateLicensed
	a.Touch(now)
	return nil
}

func permitBaselineSnapshot(b ProtectionBaseline) PermitBaselineSnapshot {
	payload := baselineDigestPayload{BaselineID: b.ID, CoordinateSystem: b.CoordinateSystem, SegmentPolygon: clonePolygon(b.SegmentPolygon), ExclusionZones: clonePolygons(b.ExclusionZones), ReferencePoints: append([]Point(nil), b.ReferencePoints...), BufferDistanceMM: b.BufferDistanceMM, MinimumSpacingMM: b.MinimumSpacingMM, MaxCoreVolumeMM3: b.MaxCoreVolumeMM3, MaxTotalVolumeMM3: b.MaxTotalVolumeMM3}
	snapshot := PermitBaselineSnapshot{BaselineID: payload.BaselineID, CoordinateSystem: payload.CoordinateSystem, SegmentPolygon: payload.SegmentPolygon, ExclusionZones: payload.ExclusionZones, ReferencePoints: payload.ReferencePoints, BufferDistanceMM: payload.BufferDistanceMM, MinimumSpacingMM: payload.MinimumSpacingMM, MaxCoreVolumeMM3: payload.MaxCoreVolumeMM3, MaxTotalVolumeMM3: payload.MaxTotalVolumeMM3}
	snapshot.ContentDigest = digestPayload(payload)
	return snapshot
}

func baselineSnapshotDigest(snapshot PermitBaselineSnapshot) string {
	return digestPayload(baselineDigestPayload{BaselineID: snapshot.BaselineID, CoordinateSystem: snapshot.CoordinateSystem, SegmentPolygon: snapshot.SegmentPolygon, ExclusionZones: snapshot.ExclusionZones, ReferencePoints: snapshot.ReferencePoints, BufferDistanceMM: snapshot.BufferDistanceMM, MinimumSpacingMM: snapshot.MinimumSpacingMM, MaxCoreVolumeMM3: snapshot.MaxCoreVolumeMM3, MaxTotalVolumeMM3: snapshot.MaxTotalVolumeMM3})
}

func permitPayload(permit SamplingPermit) permitDigestPayload {
	sites := append([]PermitSiteSnapshot(nil), permit.ApprovedSites...)
	sort.Slice(sites, func(i, j int) bool { return sites[i].SiteID < sites[j].SiteID })
	ids := append([]string(nil), permit.ApprovedSiteIDs...)
	sort.Strings(ids)
	return permitDigestPayload{PermitID: permit.ID, ApplicationID: permit.ApplicationID, ApplicationVersion: permit.ApplicationVersion, Baseline: permit.Baseline, ApprovedSiteIDs: ids, ApprovedSites: sites, FinalReviewRound: permit.FinalReviewRound, ReviewerID: permit.ReviewerID, IssuedAt: permit.IssuedAt.UTC(), Status: permit.Status}
}

func VerifyPermitDigest(permit SamplingPermit) bool {
	return permit.ContentDigest == digestPayload(permitPayload(permit)) && permit.Baseline.ContentDigest == baselineSnapshotDigest(permit.Baseline)
}

func VerifyPermit(permit SamplingPermit, app *Application) PermitVerification {
	checks := PermitVerificationChecks{}
	checks.ContentDigest = permit.ContentDigest == digestPayload(permitPayload(permit))
	checks.BaselineDigest = permit.Baseline.ContentDigest == baselineSnapshotDigest(permit.Baseline)
	checks.UniqueOwnership = app != nil && app.Permit != nil && app.Permit.ID == permit.ID && app.ID == permit.ApplicationID
	checks.ApplicationVersion = app != nil && app.Version == permit.ApplicationVersion && app.Permit != nil && app.Permit.ApplicationVersion == permit.ApplicationVersion
	checks.ApprovedSiteScope = app != nil && permitScopeMatches(permit, app)
	checks.FinalReviewRound = app != nil && permitReviewMatches(permit, app)
	verified := permit.Status == "valid" && checks.ContentDigest && checks.BaselineDigest && checks.UniqueOwnership && checks.ApplicationVersion && checks.ApprovedSiteScope && checks.FinalReviewRound
	result := PermitVerification{Verified: verified, Status: "invalid", Permit: &permit, Checks: checks, ContentSummary: permit.ContentDigest}
	if verified {
		result.Status = "valid"
		result.Scope = &PermitScope{ApplicationID: permit.ApplicationID, BaselineID: permit.Baseline.BaselineID, ApprovedSites: permit.ApprovedSites, FinalReviewRound: permit.FinalReviewRound}
	}
	return result
}

func permitScopeMatches(permit SamplingPermit, app *Application) bool {
	if app.Baseline == nil || app.Baseline.ID != permit.Baseline.BaselineID || len(permit.ApprovedSites) != len(permit.ApprovedSiteIDs) {
		return false
	}
	active := app.ActiveSites()
	if len(active) != len(permit.ApprovedSites) {
		return false
	}
	byID := map[string]*CandidateSite{}
	for _, site := range active {
		byID[site.ID] = site
	}
	for i, snapshot := range permit.ApprovedSites {
		if i >= len(permit.ApprovedSiteIDs) || permit.ApprovedSiteIDs[i] != snapshot.SiteID {
			return false
		}
		site := byID[snapshot.SiteID]
		if site == nil || site.Status != SiteApproved || site.Revision != snapshot.Revision || site.Position != snapshot.Position || site.DiameterMM != snapshot.DiameterMM || site.DepthMM != snapshot.DepthMM || site.EstimatedVolumeMM3 != snapshot.EstimatedVolumeMM3 {
			return false
		}
	}
	return true
}

func permitReviewMatches(permit SamplingPermit, app *Application) bool {
	if permit.FinalReviewRound <= 0 || permit.FinalReviewRound > len(app.ReviewRounds) {
		return false
	}
	round := app.ReviewRounds[permit.FinalReviewRound-1]
	return round.Round == permit.FinalReviewRound && round.Completed && round.ReviewerID == permit.ReviewerID
}

func digestPayload(payload any) string {
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
