package application

import (
	"cave-sampling-permit/internal/domain"
)

type CreateApplicationCommand struct {
	CaveName, SegmentName, ResearchPurpose string
	ApplicantID, ProtectionOwnerID         string
	IdempotencyKey, CorrelationID          string
}

type FreezeBaselineCommand struct {
	ApplicationID                          string
	ExpectedVersion                        int64
	ActorID, IdempotencyKey, CorrelationID string
	CoordinateSystem                       string
	SegmentPolygon                         domain.Polygon
	ExclusionZones                         []domain.Polygon
	ReferencePoints                        []domain.Point
	BufferDistanceMM, MinimumSpacingMM     float64
	MaxCoreVolumeMM3, MaxTotalVolumeMM3    float64
}

type SubmitSitesCommand struct {
	ApplicationID                          string
	ExpectedVersion                        int64
	ActorID, IdempotencyKey, CorrelationID string
	Sites                                  []domain.SiteInput
}

type RunChecksCommand struct {
	ApplicationID                          string
	ExpectedVersion                        int64
	ActorID, IdempotencyKey, CorrelationID string
}

type RemediateSiteCommand struct {
	ApplicationID, SiteID, Action          string
	ExpectedVersion                        int64
	ActorID, IdempotencyKey, CorrelationID string
	Replacement                            *domain.SiteInput
}

type BatchRemediationCommand struct {
	ApplicationID                          string
	ExpectedVersion                        int64
	ActorID, IdempotencyKey, CorrelationID string
	Actions                                []domain.RemediationAction
}

type TargetedCheckCommand struct {
	ApplicationID, SiteID                  string
	ExpectedVersion                        int64
	ActorID, IdempotencyKey, CorrelationID string
	Rules                                  []domain.RuleCode
}

type ReviewCommand struct {
	ApplicationID, ReviewerID     string
	ExpectedVersion               int64
	IdempotencyKey, CorrelationID string
	Decisions                     []domain.SiteDecision
}

type IssuePermitCommand struct {
	ApplicationID, ActorID        string
	ExpectedVersion               int64
	IdempotencyKey, CorrelationID string
}
