package application_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/storage"
)

type fixedClock struct{ value time.Time }

func (c fixedClock) Now() time.Time { return c.value }

type sequenceIDs struct{ value int }

func (s *sequenceIDs) New(prefix string) string {
	s.value++
	return fmt.Sprintf("%s-%d", prefix, s.value)
}

func TestOptimisticConcurrencyAndIdempotency(t *testing.T) {
	repo, err := storage.Open(filepath.Join(t.TempDir(), "service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := application.NewService(repo, fixedClock{time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)}, &sequenceIDs{})
	ctx := context.Background()
	create := application.CreateApplicationCommand{CaveName: "洞", SegmentName: "段", ResearchPurpose: "研究", ApplicantID: "a", ProtectionOwnerID: "o", IdempotencyKey: "create", CorrelationID: "first"}
	app, err := service.CreateApplication(ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	create.CorrelationID = "retry-correlation"
	replayed, err := service.CreateApplication(ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != app.ID {
		t.Fatalf("idempotent replay created another application: %s != %s", replayed.ID, app.ID)
	}
	create.CaveName = "另一个洞"
	_, err = service.CreateApplication(ctx, create)
	var idem *application.IdempotencyConflictError
	if !errors.As(err, &idem) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
	_, err = service.SubmitSites(ctx, application.SubmitSitesCommand{ApplicationID: app.ID, ExpectedVersion: 1, ActorID: "a", IdempotencyKey: "premature"})
	if err == nil {
		t.Fatal("submission before baseline must fail")
	}
}
