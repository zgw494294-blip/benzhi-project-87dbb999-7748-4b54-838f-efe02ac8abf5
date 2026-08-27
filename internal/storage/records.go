package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/domain"
)

func writeAudit(ctx context.Context, tx *sql.Tx, event application.AuditEvent) error {
	details, err := json.Marshal(event.Details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(application_id, event_type, actor_id, correlation_id, occurred_at, details_json) VALUES(?,?,?,?,?,?)`, event.ApplicationID, event.EventType, event.ActorID, event.CorrelationID, event.OccurredAt.Format(timeFormat), details)
	return err
}

func writeIdempotency(ctx context.Context, tx *sql.Tx, record application.IdempotencyRecord) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO idempotency_results(scope, idempotency_key, operation, fingerprint, response_json, created_at) VALUES(?,?,?,?,?,?)`, record.Scope, record.Key, record.Operation, record.Fingerprint, record.Response, record.CreatedAt.Format(timeFormat))
	return err
}

func (r *SQLiteRepository) GetIdempotency(ctx context.Context, scope, key string) (*application.IdempotencyRecord, error) {
	var record application.IdempotencyRecord
	var created string
	err := r.db.QueryRowContext(ctx, `SELECT scope, idempotency_key, operation, fingerprint, response_json, created_at FROM idempotency_results WHERE scope=? AND idempotency_key=?`, scope, key).Scan(&record.Scope, &record.Key, &record.Operation, &record.Fingerprint, &record.Response, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, application.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	parsed, err := parseTime(created)
	if err != nil {
		return nil, err
	}
	record.CreatedAt = parsed
	return &record, nil
}

func (r *SQLiteRepository) GetPermit(ctx context.Context, id string) (*domain.SamplingPermit, error) {
	r.permitStatementMu.Lock()
	defer r.permitStatementMu.Unlock()

	var tx *sql.Tx
	if r.permitStatement == nil {
		var err error
		tx, err = r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()
		r.permitStatement, err = tx.PrepareContext(ctx, "SELECT permit_json FROM permits WHERE permit_id=?")
		if err != nil {
			return nil, err
		}
	}

	var data []byte
	err := r.permitStatement.QueryRowContext(ctx, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		if tx != nil {
			_ = tx.Commit()
		}
		return nil, application.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}
	var permit domain.SamplingPermit
	if err := json.Unmarshal(data, &permit); err != nil {
		return nil, fmt.Errorf("decode permit: %w", err)
	}
	return &permit, nil
}
