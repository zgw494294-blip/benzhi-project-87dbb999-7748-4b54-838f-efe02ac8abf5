package domain

import "time"

type ApplicationState string

const (
	StateDraft          ApplicationState = "draft"
	StateBaselineFrozen ApplicationState = "baseline_frozen"
	StatePlanPending    ApplicationState = "plan_pending_check"
	StateRemediation    ApplicationState = "remediation"
	StatePendingReview  ApplicationState = "pending_review"
	StateLicensed       ApplicationState = "licensed"
	StateRejected       ApplicationState = "rejected"
)

type SiteStatus string

const (
	SiteSubmitted SiteStatus = "submitted"
	SitePassed    SiteStatus = "passed"
	SiteFailed    SiteStatus = "failed"
	SiteWithdrawn SiteStatus = "withdrawn"
	SiteApproved  SiteStatus = "approved"
	SiteRejected  SiteStatus = "rejected"
	SiteReplaced  SiteStatus = "replaced"
)

type RuleCode string

const (
	RuleBoundary   RuleCode = "boundary"
	RuleBuffer     RuleCode = "buffer"
	RuleSpacing    RuleCode = "spacing"
	RuleCoreVolume RuleCode = "core_volume"
	RuleTotal      RuleCode = "total_volume"
)

type Point struct {
	X float64 `json:"x_mm"`
	Y float64 `json:"y_mm"`
}

type Polygon struct {
	Points []Point `json:"points"`
}

type Application struct {
	ID                 string              `json:"application_id"`
	CaveName           string              `json:"cave_name"`
	SegmentName        string              `json:"segment_name"`
	ResearchPurpose    string              `json:"research_purpose"`
	ApplicantID        string              `json:"applicant_id"`
	ProtectionOwnerID  string              `json:"protection_owner_id"`
	State              ApplicationState    `json:"state"`
	Version            int64               `json:"version"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
	Baseline           *ProtectionBaseline `json:"baseline,omitempty"`
	Sites              []CandidateSite     `json:"sites,omitempty"`
	CheckBatches       []CheckBatch        `json:"check_batches,omitempty"`
	RemediationBatches []RemediationBatch  `json:"remediation_batches,omitempty"`
	Review             *IndependentReview  `json:"review,omitempty"`
	ReviewRounds       []ReviewRound       `json:"review_rounds,omitempty"`
	Permit             *SamplingPermit     `json:"permit,omitempty"`
}

type ProtectionBaseline struct {
	ID                string    `json:"baseline_id"`
	ApplicationID     string    `json:"application_id"`
	CoordinateSystem  string    `json:"coordinate_system"`
	SegmentPolygon    Polygon   `json:"segment_polygon"`
	ExclusionZones    []Polygon `json:"exclusion_zones"`
	ReferencePoints   []Point   `json:"reference_points"`
	BufferDistanceMM  float64   `json:"buffer_distance_mm"`
	MinimumSpacingMM  float64   `json:"minimum_spacing_mm"`
	MaxCoreVolumeMM3  float64   `json:"max_core_volume_mm3"`
	MaxTotalVolumeMM3 float64   `json:"max_total_volume_mm3"`
	FrozenAt          time.Time `json:"frozen_at"`
}

type CandidateSite struct {
	ID                 string        `json:"site_id"`
	ApplicationID      string        `json:"application_id"`
	Revision           int           `json:"revision"`
	ReplacesSiteID     string        `json:"replaces_site_id,omitempty"`
	Position           Point         `json:"position"`
	DiameterMM         float64       `json:"diameter_mm"`
	DepthMM            float64       `json:"depth_mm"`
	EstimatedVolumeMM3 float64       `json:"estimated_volume_mm3"`
	NecessityNote      string        `json:"necessity_note"`
	Status             SiteStatus    `json:"status"`
	CheckResults       []CheckResult `json:"check_results,omitempty"`
}

type CheckResult struct {
	ID                 string    `json:"check_result_id"`
	SiteID             string    `json:"site_id"`
	Rule               RuleCode  `json:"rule"`
	Passed             bool      `json:"passed"`
	Message            string    `json:"message"`
	CheckedAt          time.Time `json:"checked_at"`
	BatchID            string    `json:"batch_id,omitempty"`
	ApplicationVersion int64     `json:"application_version,omitempty"`
	BaselineID         string    `json:"baseline_id,omitempty"`
	SiteRevision       int       `json:"site_revision,omitempty"`
	Current            bool      `json:"current"`
}

type RuleSummary struct {
	Rule              RuleCode `json:"rule"`
	PassedCount       int      `json:"passed_count"`
	FailedCount       int      `json:"failed_count"`
	AffectedSiteCount int      `json:"affected_site_count"`
}

type CheckBatch struct {
	ID                       string        `json:"batch_id"`
	ApplicationVersion       int64         `json:"application_version"`
	BaselineID               string        `json:"baseline_id"`
	Type                     string        `json:"type"`
	SiteIDs                  []string      `json:"site_ids"`
	Rules                    []RuleCode    `json:"rules"`
	TriggeredBy              string        `json:"triggered_by"`
	CompletedAt              time.Time     `json:"completed_at"`
	ActiveSiteCount          int           `json:"active_site_count"`
	ActiveVolumeMM3          float64       `json:"active_volume_mm3"`
	VolumeLimitDifferenceMM3 float64       `json:"volume_limit_difference_mm3"`
	Summaries                []RuleSummary `json:"rule_summaries"`
	Results                  []CheckResult `json:"results"`
	Current                  bool          `json:"current"`
	HistoricalSnapshot       bool          `json:"historical_snapshot"`
}

type RemediationAction struct {
	SiteID      string     `json:"site_id"`
	Action      string     `json:"action"`
	Replacement *SiteInput `json:"replacement,omitempty"`
}

type RemediationBatch struct {
	ID               string     `json:"remediation_batch_id"`
	Actions          int        `json:"action_count"`
	SuggestedSiteIDs []string   `json:"suggested_site_ids"`
	SuggestedRules   []RuleCode `json:"suggested_rules"`
	CompletedAt      time.Time  `json:"completed_at"`
}

type SiteDecision struct {
	SiteID   string `json:"site_id"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

type IndependentReview struct {
	ReviewerID string         `json:"reviewer_id"`
	Decisions  []SiteDecision `json:"decisions"`
	Completed  bool           `json:"completed"`
	ReviewedAt time.Time      `json:"reviewed_at"`
}

type ReviewSiteSnapshot struct {
	SiteID   string `json:"site_id"`
	Revision int    `json:"revision"`
}

type ReviewRound struct {
	Round               int                  `json:"round"`
	ApplicationVersion  int64                `json:"application_version"`
	Sites               []ReviewSiteSnapshot `json:"sites"`
	ReviewerID          string               `json:"reviewer_id"`
	Decisions           []SiteDecision       `json:"decisions"`
	Completed           bool                 `json:"completed"`
	CurrentSigningBasis bool                 `json:"current_signing_basis"`
	ReviewedAt          time.Time            `json:"reviewed_at"`
}

type PermitBaselineSnapshot struct {
	BaselineID        string    `json:"baseline_id"`
	CoordinateSystem  string    `json:"coordinate_system"`
	SegmentPolygon    Polygon   `json:"segment_polygon"`
	ExclusionZones    []Polygon `json:"exclusion_zones"`
	ReferencePoints   []Point   `json:"reference_points"`
	BufferDistanceMM  float64   `json:"buffer_distance_mm"`
	MinimumSpacingMM  float64   `json:"minimum_spacing_mm"`
	MaxCoreVolumeMM3  float64   `json:"max_core_volume_mm3"`
	MaxTotalVolumeMM3 float64   `json:"max_total_volume_mm3"`
	ContentDigest     string    `json:"content_digest"`
}

type PermitSiteSnapshot struct {
	SiteID             string  `json:"site_id"`
	Revision           int     `json:"revision"`
	Position           Point   `json:"position"`
	DiameterMM         float64 `json:"diameter_mm"`
	DepthMM            float64 `json:"depth_mm"`
	EstimatedVolumeMM3 float64 `json:"estimated_volume_mm3"`
}

type SamplingPermit struct {
	ID                 string                 `json:"permit_id"`
	ApplicationID      string                 `json:"application_id"`
	ApplicationVersion int64                  `json:"application_version"`
	ApprovedSiteIDs    []string               `json:"approved_site_ids"`
	ReviewerID         string                 `json:"reviewer_id"`
	IssuedAt           time.Time              `json:"issued_at"`
	ContentDigest      string                 `json:"content_digest"`
	Status             string                 `json:"status"`
	Baseline           PermitBaselineSnapshot `json:"baseline"`
	ApprovedSites      []PermitSiteSnapshot   `json:"approved_sites"`
	FinalReviewRound   int                    `json:"final_review_round"`
}
