package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cave-sampling-permit/internal/domain"
)

type selfcheckClient struct {
	base string
	http *http.Client
}

func executeSelfcheck(ctx context.Context, baseURL string) error {
	c := &selfcheckClient{base: baseURL, http: &http.Client{Timeout: 4 * time.Second}}
	var app domain.Application
	if err := c.call(ctx, http.MethodPost, "/api/v1/sampling-applications", "", map[string]any{"cave_name": "自检洞穴", "segment_name": "北段", "research_purpose": "年代学微量取样", "applicant_id": "researcher-selfcheck", "protection_owner_id": "owner-selfcheck", "idempotency_key": "self-create"}, http.StatusCreated, &app); err != nil {
		return err
	}
	appPath := "/api/v1/sampling-applications/" + app.ID
	baseline := map[string]any{
		"expected_version": app.Version, "idempotency_key": "self-baseline", "coordinate_system": "local-mm",
		"segment_polygon": polygon(0, 0, 1000, 1000), "exclusion_zones": []any{polygon(400, 400, 500, 500)},
		"reference_points": []any{map[string]any{"x_mm": 0, "y_mm": 0}}, "buffer_distance_mm": 50,
		"minimum_spacing_mm": 100, "max_core_volume_mm3": 200, "max_total_volume_mm3": 300,
	}
	if err := c.call(ctx, http.MethodPost, appPath+"/baseline:freeze", "owner-selfcheck", baseline, http.StatusOK, &app); err != nil {
		return err
	}
	sites := map[string]any{"expected_version": app.Version, "idempotency_key": "self-sites", "sites": []any{
		map[string]any{"site_id": "site-safe", "position": map[string]any{"x_mm": 150, "y_mm": 150}, "diameter_mm": 5, "depth_mm": 10, "estimated_volume_mm3": 100, "necessity_note": "建立年代锚点"},
		map[string]any{"site_id": "site-risk", "position": map[string]any{"x_mm": 450, "y_mm": 450}, "diameter_mm": 5, "depth_mm": 10, "estimated_volume_mm3": 100, "necessity_note": "补充层序对照"},
	}}
	if err := c.call(ctx, http.MethodPost, appPath+"/sites:submit", "researcher-selfcheck", sites, http.StatusOK, &app); err != nil {
		return err
	}
	if err := c.call(ctx, http.MethodPost, appPath+"/checks:run", "owner-selfcheck", mutation(app.Version, "self-checks"), http.StatusOK, &app); err != nil {
		return err
	}
	if app.State != domain.StateRemediation {
		return fmt.Errorf("expected remediation after exclusion-zone violation, got %s", app.State)
	}
	replacement := map[string]any{"site_id": "site-risk-r2", "position": map[string]any{"x_mm": 700, "y_mm": 700}, "diameter_mm": 5, "depth_mm": 10, "estimated_volume_mm3": 100, "necessity_note": "移出禁采区后的层序对照"}
	remediation := map[string]any{"expected_version": app.Version, "idempotency_key": "self-revise", "action": "revise", "replacement": replacement}
	if err := c.call(ctx, http.MethodPost, appPath+"/sites/site-risk/remediate", "researcher-selfcheck", remediation, http.StatusOK, &app); err != nil {
		return err
	}
	if err := c.call(ctx, http.MethodPost, appPath+"/sites/site-risk-r2/checks:run", "owner-selfcheck", map[string]any{"expected_version": app.Version, "idempotency_key": "self-targeted", "rules": []string{"boundary", "buffer", "spacing", "core_volume", "total_volume"}}, http.StatusOK, &app); err != nil {
		return err
	}
	if app.State != domain.StatePendingReview {
		return fmt.Errorf("expected pending_review after targeted checks, got %s", app.State)
	}
	review := map[string]any{"expected_version": app.Version, "idempotency_key": "self-review", "reviewer_id": "reviewer-independent", "decisions": []any{map[string]any{"site_id": "site-safe", "decision": "approve"}, map[string]any{"site_id": "site-risk-r2", "decision": "approve"}}}
	if err := c.call(ctx, http.MethodPost, appPath+"/reviews", "reviewer-independent", review, http.StatusOK, &app); err != nil {
		return err
	}
	var permit domain.SamplingPermit
	if err := c.call(ctx, http.MethodPost, appPath+"/permit:issue", "owner-selfcheck", mutation(app.Version, "self-permit"), http.StatusCreated, &permit); err != nil {
		return err
	}
	var verification struct {
		Verified bool                  `json:"verified"`
		Permit   domain.SamplingPermit `json:"permit"`
	}
	if err := c.call(ctx, http.MethodGet, "/api/v1/permits/"+permit.ID, "", nil, http.StatusOK, &verification); err != nil {
		return err
	}
	if !verification.Verified || verification.Permit.ContentDigest != permit.ContentDigest {
		return fmt.Errorf("permit verification did not return matching digest")
	}
	return nil
}

func mutation(version int64, key string) map[string]any {
	return map[string]any{"expected_version": version, "idempotency_key": key}
}
func polygon(minX, minY, maxX, maxY float64) map[string]any {
	return map[string]any{"points": []any{map[string]any{"x_mm": minX, "y_mm": minY}, map[string]any{"x_mm": maxX, "y_mm": minY}, map[string]any{"x_mm": maxX, "y_mm": maxY}, map[string]any{"x_mm": minX, "y_mm": maxY}}}
}

func (c *selfcheckClient) call(ctx context.Context, method, path, actor string, body any, expected int, output any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("X-Actor-ID", actor)
	}
	req.Header.Set("X-Correlation-ID", "selfcheck")
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != expected {
		return fmt.Errorf("%s %s returned %d, expected %d: %s", method, path, response.StatusCode, expected, strings.TrimSpace(string(data)))
	}
	if output != nil && len(data) != 0 {
		if err := json.Unmarshal(data, output); err != nil {
			return fmt.Errorf("decode %s %s response: %w", method, path, err)
		}
	}
	return nil
}
