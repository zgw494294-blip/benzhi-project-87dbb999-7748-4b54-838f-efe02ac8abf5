package concurrent_idempotent_mutation_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/domain"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type fixedIDs struct{}

func (fixedIDs) New(prefix string) string { return prefix + "-fixed" }

type barrierRepository struct {
	mu            sync.Mutex
	app           *domain.Application
	idempotency   *application.IdempotencyRecord
	replayCalls   int
	replaysReady  chan struct{}
	replaysPassed chan struct{}
}

func newBarrierRepository(app *domain.Application) *barrierRepository {
	return &barrierRepository{app: cloneApplication(app), replaysReady: make(chan struct{}), replaysPassed: make(chan struct{})}
}

func (r *barrierRepository) Create(context.Context, *domain.Application, application.AuditEvent, application.IdempotencyRecord) error {
	return errors.New("unexpected Create")
}

func (r *barrierRepository) Get(_ context.Context, _ string) (*domain.Application, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneApplication(r.app), nil
}

func (r *barrierRepository) Update(_ context.Context, app *domain.Application, expected int64, _ application.AuditEvent, record application.IdempotencyRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.app.Version != expected {
		return &application.VersionConflictError{Expected: expected, Actual: r.app.Version}
	}
	app.Version = expected + 1
	r.app = cloneApplication(app)
	record.Response, _ = json.Marshal(app)
	copy := record
	r.idempotency = &copy
	return nil
}

func (r *barrierRepository) GetIdempotency(_ context.Context, _, _ string) (*application.IdempotencyRecord, error) {
	r.mu.Lock()
	if r.idempotency != nil {
		copy := *r.idempotency
		r.mu.Unlock()
		return &copy, nil
	}
	r.replayCalls++
	if r.replayCalls == 2 {
		close(r.replaysReady)
	}
	r.mu.Unlock()
	<-r.replaysReady

	r.mu.Lock()
	r.replayCalls--
	if r.replayCalls == 0 {
		close(r.replaysPassed)
	}
	r.mu.Unlock()
	<-r.replaysPassed
	return nil, application.ErrNotFound
}

func (r *barrierRepository) GetPermit(context.Context, string) (*domain.SamplingPermit, error) {
	return nil, application.ErrNotFound
}

func (r *barrierRepository) Close() error { return nil }

func cloneApplication(app *domain.Application) *domain.Application {
	data, _ := json.Marshal(app)
	var copy domain.Application
	_ = json.Unmarshal(data, &copy)
	return &copy
}

func TestConcurrentIdenticalMutationReplaysWinner(t *testing.T) {
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	app, err := domain.CreateApplication(domain.NewApplication{ID: "app-1", CaveName: "洞", SegmentName: "段", ResearchPurpose: "研究", ApplicantID: "applicant", ProtectionOwnerID: "owner", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	repo := newBarrierRepository(app)
	service := application.NewService(repo, fixedClock{now: now}, fixedIDs{})
	cmd := application.FreezeBaselineCommand{
		ApplicationID: "app-1", ExpectedVersion: 1, ActorID: "owner", IdempotencyKey: "same-key",
		CoordinateSystem: "local-mm",
		SegmentPolygon:   domain.Polygon{Points: []domain.Point{{X: 0, Y: 0}, {X: 1000, Y: 0}, {X: 1000, Y: 1000}, {X: 0, Y: 1000}}},
		ReferencePoints:  []domain.Point{{X: 100, Y: 100}}, MinimumSpacingMM: 10,
		MaxCoreVolumeMM3: 100, MaxTotalVolumeMM3: 1000,
	}

	start := make(chan struct{})
	errorsSeen := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer workers.Done()
			<-start
			_, callErr := service.FreezeBaseline(context.Background(), cmd)
			errorsSeen <- callErr
		}()
	}
	close(start)
	workers.Wait()
	close(errorsSeen)
	for callErr := range errorsSeen {
		if callErr != nil {
			t.Fatalf("an identical in-flight retry must replay the committed response, got %v", callErr)
		}
	}
}
