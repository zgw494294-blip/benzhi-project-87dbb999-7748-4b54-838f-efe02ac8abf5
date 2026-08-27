package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"cave-sampling-permit/internal/domain"
)

const timeFormat = "2006-01-02T15:04:05.999999999Z07:00"

func writeSnapshot(ctx context.Context, tx *sql.Tx, app *domain.Application) error {
	if app.Baseline != nil {
		data, _ := json.Marshal(app.Baseline)
		_, err := tx.ExecContext(ctx, `INSERT INTO protection_baselines(baseline_id, application_id, baseline_json, frozen_at) VALUES(?,?,?,?) ON CONFLICT(application_id) DO UPDATE SET baseline_json=excluded.baseline_json`, app.Baseline.ID, app.ID, data, app.Baseline.FrozenAt.Format(timeFormat))
		if err != nil {
			return fmt.Errorf("persist baseline: %w", err)
		}
	}
	for i := range app.Sites {
		site := &app.Sites[i]
		data, _ := json.Marshal(site)
		var replaces any
		if site.ReplacesSiteID != "" {
			replaces = site.ReplacesSiteID
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO candidate_sites(site_id, application_id, revision, replaces_site_id, status, site_json) VALUES(?,?,?,?,?,?) ON CONFLICT(site_id) DO UPDATE SET status=excluded.status, site_json=excluded.site_json`, site.ID, app.ID, site.Revision, replaces, site.Status, data)
		if err != nil {
			return fmt.Errorf("persist candidate site: %w", err)
		}
		for _, check := range site.CheckResults {
			resultData, _ := json.Marshal(check)
			passed := 0
			if check.Passed {
				passed = 1
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO check_results(check_result_id, site_id, rule_code, passed, result_json) VALUES(?,?,?,?,?) ON CONFLICT(check_result_id) DO UPDATE SET passed=excluded.passed, result_json=excluded.result_json`, check.ID, site.ID, check.Rule, passed, resultData)
			if err != nil {
				return fmt.Errorf("persist check result: %w", err)
			}
		}
	}
	for i := range app.CheckBatches {
		batch := &app.CheckBatches[i]
		data, _ := json.Marshal(batch)
		_, err := tx.ExecContext(ctx, `INSERT INTO check_batches(batch_id, application_id, application_version, baseline_id, batch_type, batch_json, completed_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(batch_id) DO NOTHING`, batch.ID, app.ID, batch.ApplicationVersion, batch.BaselineID, batch.Type, data, batch.CompletedAt.Format(timeFormat))
		if err != nil {
			return fmt.Errorf("persist check batch: %w", err)
		}
	}
	for i := range app.ReviewRounds {
		round := &app.ReviewRounds[i]
		data, _ := json.Marshal(round)
		_, err := tx.ExecContext(ctx, `INSERT INTO review_rounds(application_id, round_number, application_version, review_json, reviewed_at) VALUES(?,?,?,?,?) ON CONFLICT(application_id, round_number) DO NOTHING`, app.ID, round.Round, round.ApplicationVersion, data, round.ReviewedAt.Format(timeFormat))
		if err != nil {
			return fmt.Errorf("persist review round: %w", err)
		}
	}
	if app.Permit != nil {
		data, _ := json.Marshal(app.Permit)
		_, err := tx.ExecContext(ctx, `INSERT INTO permits(permit_id, application_id, content_digest, status, permit_json, issued_at) VALUES(?,?,?,?,?,?)`, app.Permit.ID, app.ID, app.Permit.ContentDigest, app.Permit.Status, data, app.Permit.IssuedAt.Format(timeFormat))
		if err != nil {
			return fmt.Errorf("persist unique permit: %w", err)
		}
	}
	return nil
}
