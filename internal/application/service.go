package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"cave-sampling-permit/internal/domain"
)

type Service struct {
	repo  Repository
	clock Clock
	ids   IDGenerator
}

func NewService(repo Repository, clock Clock, ids IDGenerator) *Service {
	return &Service{repo: repo, clock: clock, ids: ids}
}

func (s *Service) GetApplication(ctx context.Context, id string) (*domain.Application, error) {
	app, err := s.repo.Get(ctx, id)
	if err == nil {
		markCheckBatchHistory(app)
	}
	return app, err
}

func (s *Service) CreateApplication(ctx context.Context, cmd CreateApplicationCommand) (*domain.Application, error) {
	if cmd.IdempotencyKey == "" {
		return nil, validation("idempotency_key", "required", "幂等键不能为空")
	}
	scope := "create:" + cmd.ApplicantID
	fingerprint := requestFingerprint("create_application", cmd)
	if replay, err := s.replay(ctx, scope, cmd.IdempotencyKey, fingerprint); err != nil || replay != nil {
		return replay, err
	}
	now := s.clock.Now()
	app, err := domain.CreateApplication(domain.NewApplication{ID: s.ids.New("app"), CaveName: cmd.CaveName, SegmentName: cmd.SegmentName, ResearchPurpose: cmd.ResearchPurpose, ApplicantID: cmd.ApplicantID, ProtectionOwnerID: cmd.ProtectionOwnerID, Now: now})
	if err != nil {
		return nil, err
	}
	record := s.makeIdempotency(scope, cmd.IdempotencyKey, "create_application", fingerprint, now)
	event := AuditEvent{ApplicationID: app.ID, EventType: "application_created", ActorID: cmd.ApplicantID, CorrelationID: cmd.CorrelationID, OccurredAt: now, Details: map[string]any{"state": app.State}}
	if err := s.repo.Create(ctx, app, event, record); err != nil {
		// A concurrent identical request may have committed the same idempotency
		// record between the replay check and this write. Re-check and replay the
		// stored response so a safe retry does not surface the persistence error.
		if replay, replayErr := s.replay(ctx, scope, cmd.IdempotencyKey, fingerprint); replayErr != nil {
			return nil, replayErr
		} else if replay != nil {
			return replay, nil
		}
		return nil, err
	}
	return app, nil
}

func (s *Service) mutate(ctx context.Context, appID string, expected int64, actor, key, correlation, operation, fingerprint string, action func(*domain.Application) error) (*domain.Application, error) {
	if key == "" {
		return nil, validation("idempotency_key", "required", "幂等键不能为空")
	}
	if replay, err := s.replay(ctx, appID, key, fingerprint); err != nil || replay != nil {
		return replay, err
	}
	app, err := s.repo.Get(ctx, appID)
	if err != nil {
		return nil, err
	}
	if app.Version != expected {
		// A concurrent identical request may have committed the same idempotency
		// record between the replay check and this read. Re-check and replay the
		// stored response so a safe retry does not surface a version conflict.
		if replay, replayErr := s.replay(ctx, appID, key, fingerprint); replayErr != nil {
			return nil, replayErr
		} else if replay != nil {
			return replay, nil
		}
		return nil, &VersionConflictError{Expected: expected, Actual: app.Version}
	}
	if err := action(app); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	record := s.makeIdempotency(appID, key, operation, fingerprint, now)
	event := AuditEvent{ApplicationID: appID, EventType: operation, ActorID: actor, CorrelationID: correlation, OccurredAt: now, Details: map[string]any{"from_version": expected, "state": app.State}}
	if operation == "sites_remediated" && len(app.RemediationBatches) > 0 {
		batch := app.RemediationBatches[len(app.RemediationBatches)-1]
		event.Details["remediation_count"] = batch.Actions
		event.Details["suggested_site_ids"] = batch.SuggestedSiteIDs
		event.Details["suggested_rules"] = batch.SuggestedRules
	}
	if err := s.repo.Update(ctx, app, expected, event, record); err != nil {
		// A concurrent identical request may have advanced the aggregate version
		// and stored the same idempotency record between the replay check and this
		// write. Re-check and replay the stored response so a safe retry does not
		// surface a version conflict or persistence error.
		if replay, replayErr := s.replay(ctx, appID, key, fingerprint); replayErr != nil {
			return nil, replayErr
		} else if replay != nil {
			return replay, nil
		}
		return nil, err
	}
	markCheckBatchHistory(app)
	return app, nil
}

func (s *Service) replay(ctx context.Context, scope, key, fingerprint string) (*domain.Application, error) {
	record, err := s.repo.GetIdempotency(ctx, scope, key)
	if err != nil {
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	if record.Fingerprint != fingerprint {
		return nil, &IdempotencyConflictError{Key: key}
	}
	var app domain.Application
	if err := json.Unmarshal(record.Response, &app); err != nil {
		return nil, fmt.Errorf("decode idempotent response: %w", err)
	}
	markCheckBatchHistory(&app)
	return &app, nil
}

func markCheckBatchHistory(app *domain.Application) {
	for i := range app.CheckBatches {
		app.CheckBatches[i].HistoricalSnapshot = app.CheckBatches[i].ApplicationVersion != app.Version
		app.CheckBatches[i].Current = app.CheckBatches[i].Current && !app.CheckBatches[i].HistoricalSnapshot
	}
}

func requestFingerprint(operation string, value any) string {
	valueData, _ := json.Marshal(value)
	var stable any
	_ = json.Unmarshal(valueData, &stable)
	removeTransportMetadata(stable)
	data, _ := json.Marshal(struct {
		Operation string `json:"operation"`
		Value     any    `json:"value"`
	}{operation, stable})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func removeTransportMetadata(value any) {
	switch item := value.(type) {
	case map[string]any:
		delete(item, "IdempotencyKey")
		delete(item, "CorrelationID")
		for _, child := range item {
			removeTransportMetadata(child)
		}
	case []any:
		for _, child := range item {
			removeTransportMetadata(child)
		}
	}
}

func validation(field, code, message string) error {
	return &domain.ValidationError{Violations: []domain.Violation{{Field: field, Code: code, Message: message}}}
}
