package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/domain"
)

func TestRepositoryRestoresAggregateAndPermitAfterRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	ctx := context.Background()
	now := time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC)
	repo, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	app, _ := domain.CreateApplication(domain.NewApplication{ID: "app-restart", CaveName: "洞", SegmentName: "段", ResearchPurpose: "研究", ApplicantID: "a", ProtectionOwnerID: "o", Now: now})
	createEvent := application.AuditEvent{ApplicationID: app.ID, EventType: "created", ActorID: "a", CorrelationID: "test", OccurredAt: now}
	createIdem := application.IdempotencyRecord{Scope: "create:a", Key: "one", Operation: "created", Fingerprint: "fp", CreatedAt: now}
	if err := repo.Create(ctx, app, createEvent, createIdem); err != nil {
		t.Fatal(err)
	}
	app.State = domain.StatePendingReview
	app.Review = &domain.IndependentReview{ReviewerID: "r", Completed: true, ReviewedAt: now}
	app.Baseline = &domain.ProtectionBaseline{ID: "baseline-restart", ApplicationID: app.ID, CoordinateSystem: "local-mm", SegmentPolygon: domain.Polygon{Points: []domain.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}}}, ReferencePoints: []domain.Point{{X: 1, Y: 1}}, MinimumSpacingMM: 1, MaxCoreVolumeMM3: 10, MaxTotalVolumeMM3: 10, FrozenAt: now}
	app.ReviewRounds = []domain.ReviewRound{{Round: 1, ApplicationVersion: 1, ReviewerID: "r", Completed: true, CurrentSigningBasis: true, ReviewedAt: now}}
	app.Sites = []domain.CandidateSite{{ID: "s", ApplicationID: app.ID, Revision: 1, Status: domain.SiteApproved}}
	if err := app.IssuePermit("permit-restart", now); err != nil {
		t.Fatal(err)
	}
	updateEvent := application.AuditEvent{ApplicationID: app.ID, EventType: "permit_issued", ActorID: "o", CorrelationID: "test", OccurredAt: now}
	updateIdem := application.IdempotencyRecord{Scope: app.ID, Key: "two", Operation: "permit_issued", Fingerprint: "fp2", CreatedAt: now}
	if err := repo.Update(ctx, app, 1, updateEvent, updateIdem); err != nil {
		t.Fatal(err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored, err := reopened.Get(ctx, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Version != 2 || restored.State != domain.StateLicensed {
		t.Fatalf("restored version/state = %d/%s", restored.Version, restored.State)
	}
	permit, err := reopened.GetPermit(ctx, "permit-restart")
	if err != nil {
		t.Fatal(err)
	}
	if !domain.VerifyPermitDigest(*permit) {
		t.Fatal("restored permit digest failed")
	}
}
