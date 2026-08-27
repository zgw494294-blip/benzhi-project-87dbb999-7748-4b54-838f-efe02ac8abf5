package storage

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaV1 = `
CREATE TABLE IF NOT EXISTS schema_version (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);
CREATE TABLE applications (
  application_id TEXT PRIMARY KEY,
  version INTEGER NOT NULL CHECK (version > 0),
  state TEXT NOT NULL,
  aggregate_json BLOB NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE protection_baselines (
  baseline_id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL UNIQUE REFERENCES applications(application_id) ON DELETE RESTRICT,
  baseline_json BLOB NOT NULL,
  frozen_at TEXT NOT NULL
);
CREATE TABLE candidate_sites (
  site_id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL REFERENCES applications(application_id) ON DELETE RESTRICT,
  revision INTEGER NOT NULL CHECK (revision > 0),
  replaces_site_id TEXT REFERENCES candidate_sites(site_id) ON DELETE RESTRICT,
  status TEXT NOT NULL,
  site_json BLOB NOT NULL,
  UNIQUE(application_id, site_id)
);
CREATE TABLE check_results (
  check_result_id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES candidate_sites(site_id) ON DELETE RESTRICT,
  rule_code TEXT NOT NULL,
  passed INTEGER NOT NULL CHECK (passed IN (0, 1)),
  result_json BLOB NOT NULL
);
CREATE TABLE permits (
  permit_id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL UNIQUE REFERENCES applications(application_id) ON DELETE RESTRICT,
  content_digest TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL,
  permit_json BLOB NOT NULL,
  issued_at TEXT NOT NULL
);
CREATE TABLE idempotency_results (
  scope TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  operation TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  response_json BLOB NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(scope, idempotency_key)
);
CREATE TABLE audit_events (
  event_id INTEGER PRIMARY KEY AUTOINCREMENT,
  application_id TEXT NOT NULL REFERENCES applications(application_id) ON DELETE RESTRICT,
  event_type TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  correlation_id TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  details_json BLOB NOT NULL
);
CREATE TRIGGER audit_events_no_update BEFORE UPDATE ON audit_events BEGIN
  SELECT RAISE(ABORT, 'audit events are append only');
END;
CREATE TRIGGER audit_events_no_delete BEFORE DELETE ON audit_events BEGIN
  SELECT RAISE(ABORT, 'audit events are append only');
END;
`

const schemaV2 = `
CREATE TABLE check_batches (
  batch_id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL REFERENCES applications(application_id) ON DELETE RESTRICT,
  application_version INTEGER NOT NULL CHECK (application_version > 0),
  baseline_id TEXT NOT NULL REFERENCES protection_baselines(baseline_id) ON DELETE RESTRICT,
  batch_type TEXT NOT NULL CHECK (batch_type IN ('full', 'targeted')),
  batch_json BLOB NOT NULL,
  completed_at TEXT NOT NULL
);
CREATE TABLE review_rounds (
  application_id TEXT NOT NULL REFERENCES applications(application_id) ON DELETE RESTRICT,
  round_number INTEGER NOT NULL CHECK (round_number > 0),
  application_version INTEGER NOT NULL CHECK (application_version > 0),
  review_json BLOB NOT NULL,
  reviewed_at TEXT NOT NULL,
  PRIMARY KEY(application_id, round_number)
);
CREATE TRIGGER check_batches_no_update BEFORE UPDATE ON check_batches BEGIN
  SELECT RAISE(ABORT, 'check batches are append only');
END;
CREATE TRIGGER check_batches_no_delete BEFORE DELETE ON check_batches BEGIN
  SELECT RAISE(ABORT, 'check batches are append only');
END;
CREATE TRIGGER review_rounds_no_update BEFORE UPDATE ON review_rounds BEGIN
  SELECT RAISE(ABORT, 'review rounds are append only');
END;
CREATE TRIGGER review_rounds_no_delete BEFORE DELETE ON review_rounds BEGIN
  SELECT RAISE(ABORT, 'review rounds are append only');
END;
`

func (r *SQLiteRepository) migrate(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)"); err != nil {
		return err
	}
	var version int
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if version > 2 {
		return fmt.Errorf("database schema version %d is newer than supported version 2", version)
	}
	if version == 0 {
		if _, err := tx.ExecContext(ctx, schemaV1); err != nil {
			return fmt.Errorf("apply schema v1: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_version(version, applied_at) VALUES(1, strftime('%Y-%m-%dT%H:%M:%fZ','now'))"); err != nil {
			return err
		}
		version = 1
	}
	if version == 1 {
		if _, err := tx.ExecContext(ctx, schemaV2); err != nil {
			return fmt.Errorf("apply schema v2: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_version(version, applied_at) VALUES(2, strftime('%Y-%m-%dT%H:%M:%fZ','now'))"); err != nil {
			return err
		}
	}
	return tx.Commit()
}
