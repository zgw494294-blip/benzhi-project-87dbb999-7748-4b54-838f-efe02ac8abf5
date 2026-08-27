package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/domain"
)

func (r *SQLiteRepository) Create(ctx context.Context, app *domain.Application, event application.AuditEvent, idem application.IdempotencyRecord) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	encoded, err := json.Marshal(app)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO applications(application_id, version, state, aggregate_json, created_at, updated_at) VALUES(?,?,?,?,?,?)`, app.ID, app.Version, app.State, encoded, app.CreatedAt.Format(timeFormat), app.UpdatedAt.Format(timeFormat))
	if err != nil {
		return classifySQLite(err)
	}
	if err := writeSnapshot(ctx, tx, app); err != nil {
		return err
	}
	if err := writeAudit(ctx, tx, event); err != nil {
		return err
	}
	idem.Response = encoded
	if err := writeIdempotency(ctx, tx, idem); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (*domain.Application, error) {
	var encoded []byte
	err := r.db.QueryRowContext(ctx, "SELECT aggregate_json FROM applications WHERE application_id = ?", id).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, application.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var app domain.Application
	if err := json.Unmarshal(encoded, &app); err != nil {
		return nil, fmt.Errorf("decode application aggregate: %w", err)
	}
	return &app, nil
}

func (r *SQLiteRepository) Update(ctx context.Context, app *domain.Application, expected int64, event application.AuditEvent, idem application.IdempotencyRecord) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var actual int64
	if err := tx.QueryRowContext(ctx, "SELECT version FROM applications WHERE application_id = ?", app.ID).Scan(&actual); errors.Is(err, sql.ErrNoRows) {
		return application.ErrNotFound
	} else if err != nil {
		return err
	}
	if actual != expected {
		return &application.VersionConflictError{Expected: expected, Actual: actual}
	}
	app.Version = expected + 1
	encoded, err := json.Marshal(app)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE applications SET version=?, state=?, aggregate_json=?, updated_at=? WHERE application_id=? AND version=?`, app.Version, app.State, encoded, app.UpdatedAt.Format(timeFormat), app.ID, expected)
	if err != nil {
		return classifySQLite(err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return &application.VersionConflictError{Expected: expected, Actual: actual}
	}
	if err := writeSnapshot(ctx, tx, app); err != nil {
		return err
	}
	if err := writeAudit(ctx, tx, event); err != nil {
		return err
	}
	idem.Response = encoded
	if err := writeIdempotency(ctx, tx, idem); err != nil {
		return err
	}
	return tx.Commit()
}

func classifySQLite(err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return fmt.Errorf("unique persistence constraint: %w", err)
	}
	return err
}
