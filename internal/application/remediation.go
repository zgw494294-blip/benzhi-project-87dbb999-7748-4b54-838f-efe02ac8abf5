package application

import (
	"context"

	"cave-sampling-permit/internal/domain"
)

func (s *Service) RemediateSite(ctx context.Context, cmd RemediateSiteCommand) (*domain.Application, error) {
	fingerprint := requestFingerprint("remediate_site", cmd)
	return s.mutate(ctx, cmd.ApplicationID, cmd.ExpectedVersion, cmd.ActorID, cmd.IdempotencyKey, cmd.CorrelationID, "site_remediated", fingerprint, func(app *domain.Application) error {
		if cmd.ActorID != app.ApplicantID {
			return validation("actor_id", "forbidden", "仅申请人可整改孔位")
		}
		switch cmd.Action {
		case "withdraw":
			return app.WithdrawFailedSite(cmd.SiteID, s.clock.Now())
		case "revise":
			if cmd.Replacement == nil {
				return validation("replacement", "required", "修订操作必须提供 replacement")
			}
			return app.ReviseFailedSite(cmd.SiteID, *cmd.Replacement, s.clock.Now())
		default:
			return validation("action", "invalid_action", "整改操作必须为 withdraw 或 revise")
		}
	})
}

func (s *Service) RunTargetedChecks(ctx context.Context, cmd TargetedCheckCommand) (*domain.Application, error) {
	fingerprint := requestFingerprint("targeted_checks", cmd)
	return s.mutate(ctx, cmd.ApplicationID, cmd.ExpectedVersion, cmd.ActorID, cmd.IdempotencyKey, cmd.CorrelationID, "targeted_checks_completed", fingerprint, func(app *domain.Application) error {
		if cmd.ActorID != app.ProtectionOwnerID {
			return validation("actor_id", "forbidden", "仅保护责任人可执行定向复检")
		}
		return app.RunTargetedChecksInBatch(domain.CheckBatchInput{ID: s.ids.New("batch"), Type: "targeted", SiteID: cmd.SiteID, Rules: cmd.Rules, TriggeredBy: cmd.ActorID, Now: s.clock.Now(), ResultIDs: func(siteID string, rule domain.RuleCode) string { return s.ids.New("check") }})
	})
}

func (s *Service) RemediateSites(ctx context.Context, cmd BatchRemediationCommand) (*domain.Application, error) {
	fingerprint := requestFingerprint("remediate_sites", cmd)
	return s.mutate(ctx, cmd.ApplicationID, cmd.ExpectedVersion, cmd.ActorID, cmd.IdempotencyKey, cmd.CorrelationID, "sites_remediated", fingerprint, func(app *domain.Application) error {
		if cmd.ActorID != app.ApplicantID {
			return validation("actor_id", "forbidden", "仅申请人可整改孔位")
		}
		return app.RemediateFailedSites(domain.BatchRemediationInput{ID: s.ids.New("remediation"), Actions: cmd.Actions, Now: s.clock.Now()})
	})
}
