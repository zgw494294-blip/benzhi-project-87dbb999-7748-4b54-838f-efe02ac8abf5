package application

import (
	"context"

	"cave-sampling-permit/internal/domain"
)

func (s *Service) FreezeBaseline(ctx context.Context, cmd FreezeBaselineCommand) (*domain.Application, error) {
	fingerprint := requestFingerprint("freeze_baseline", cmd)
	return s.mutate(ctx, cmd.ApplicationID, cmd.ExpectedVersion, cmd.ActorID, cmd.IdempotencyKey, cmd.CorrelationID, "baseline_frozen", fingerprint, func(app *domain.Application) error {
		if cmd.ActorID != app.ProtectionOwnerID {
			return validation("actor_id", "forbidden", "仅保护责任人可冻结保护基线")
		}
		return app.FreezeBaseline(domain.BaselineInput{ID: s.ids.New("base"), CoordinateSystem: cmd.CoordinateSystem, SegmentPolygon: cmd.SegmentPolygon, ExclusionZones: cmd.ExclusionZones, ReferencePoints: cmd.ReferencePoints, BufferDistanceMM: cmd.BufferDistanceMM, MinimumSpacingMM: cmd.MinimumSpacingMM, MaxCoreVolumeMM3: cmd.MaxCoreVolumeMM3, MaxTotalVolumeMM3: cmd.MaxTotalVolumeMM3, Now: s.clock.Now()})
	})
}

func (s *Service) SubmitSites(ctx context.Context, cmd SubmitSitesCommand) (*domain.Application, error) {
	fingerprint := requestFingerprint("submit_sites", cmd)
	return s.mutate(ctx, cmd.ApplicationID, cmd.ExpectedVersion, cmd.ActorID, cmd.IdempotencyKey, cmd.CorrelationID, "sites_submitted", fingerprint, func(app *domain.Application) error {
		if cmd.ActorID != app.ApplicantID {
			return validation("actor_id", "forbidden", "仅申请人可提交候选孔位")
		}
		return app.SubmitSites(cmd.Sites, s.clock.Now())
	})
}

func (s *Service) RunChecks(ctx context.Context, cmd RunChecksCommand) (*domain.Application, error) {
	fingerprint := requestFingerprint("run_checks", cmd)
	return s.mutate(ctx, cmd.ApplicationID, cmd.ExpectedVersion, cmd.ActorID, cmd.IdempotencyKey, cmd.CorrelationID, "checks_completed", fingerprint, func(app *domain.Application) error {
		if cmd.ActorID != app.ProtectionOwnerID {
			return validation("actor_id", "forbidden", "仅保护责任人可执行规则检查")
		}
		return app.RunChecksInBatch(domain.CheckBatchInput{ID: s.ids.New("batch"), Type: "full", TriggeredBy: cmd.ActorID, Now: s.clock.Now(), ResultIDs: func(siteID string, rule domain.RuleCode) string { return s.ids.New("check") }})
	})
}
