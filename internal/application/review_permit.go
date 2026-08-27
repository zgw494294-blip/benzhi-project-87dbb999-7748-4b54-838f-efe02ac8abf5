package application

import (
	"context"

	"cave-sampling-permit/internal/domain"
)

func (s *Service) CompleteReview(ctx context.Context, cmd ReviewCommand) (*domain.Application, error) {
	fingerprint := requestFingerprint("complete_review", cmd)
	return s.mutate(ctx, cmd.ApplicationID, cmd.ExpectedVersion, cmd.ReviewerID, cmd.IdempotencyKey, cmd.CorrelationID, "independent_review_completed", fingerprint, func(app *domain.Application) error {
		return app.CompleteReview(cmd.ReviewerID, cmd.Decisions, s.clock.Now())
	})
}

func (s *Service) IssuePermit(ctx context.Context, cmd IssuePermitCommand) (*domain.Application, error) {
	fingerprint := requestFingerprint("issue_permit", cmd)
	return s.mutate(ctx, cmd.ApplicationID, cmd.ExpectedVersion, cmd.ActorID, cmd.IdempotencyKey, cmd.CorrelationID, "permit_issued", fingerprint, func(app *domain.Application) error {
		if cmd.ActorID != app.ProtectionOwnerID {
			return validation("actor_id", "forbidden", "仅保护责任人可签发许可证")
		}
		return app.IssuePermit(s.ids.New("permit"), s.clock.Now())
	})
}

func (s *Service) VerifyPermit(ctx context.Context, permitID string) (*domain.PermitVerification, error) {
	permit, err := s.repo.GetPermit(ctx, permitID)
	if err != nil {
		return nil, err
	}
	app, err := s.repo.Get(ctx, permit.ApplicationID)
	if err != nil {
		if err == ErrNotFound {
			result := domain.VerifyPermit(*permit, nil)
			return &result, nil
		}
		return nil, err
	}
	result := domain.VerifyPermit(*permit, app)
	return &result, nil
}
