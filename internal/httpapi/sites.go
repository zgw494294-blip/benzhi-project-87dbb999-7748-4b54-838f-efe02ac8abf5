package httpapi

import (
	"net/http"
	"strconv"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/domain"
)

type siteRequest struct {
	SiteID             string       `json:"site_id"`
	Position           pointRequest `json:"position"`
	DiameterMM         float64      `json:"diameter_mm"`
	DepthMM            float64      `json:"depth_mm"`
	EstimatedVolumeMM3 float64      `json:"estimated_volume_mm3"`
	NecessityNote      string       `json:"necessity_note"`
}

func (s siteRequest) domain() domain.SiteInput {
	return domain.SiteInput{ID: s.SiteID, Position: s.Position.domain(), DiameterMM: s.DiameterMM, DepthMM: s.DepthMM, EstimatedVolumeMM3: s.EstimatedVolumeMM3, NecessityNote: s.NecessityNote}
}

type submitSitesRequest struct {
	mutationMeta
	Sites []siteRequest `json:"sites"`
}
type checksRequest struct{ mutationMeta }
type remediationRequest struct {
	mutationMeta
	Action      string       `json:"action"`
	Replacement *siteRequest `json:"replacement,omitempty"`
}
type targetedChecksRequest struct {
	mutationMeta
	Rules []domain.RuleCode `json:"rules"`
}

type batchRemediationActionRequest struct {
	SiteID      string       `json:"site_id"`
	Action      string       `json:"action"`
	Replacement *siteRequest `json:"replacement,omitempty"`
}

type batchRemediationRequest struct {
	mutationMeta
	Actions []batchRemediationActionRequest `json:"actions"`
}

func (a *API) HandleSubmitSites(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var body submitSitesRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeDecodeError(w, err)
		return
	}
	sites := make([]domain.SiteInput, len(body.Sites))
	for i := range body.Sites {
		sites[i] = body.Sites[i].domain()
	}
	app, err := a.service.SubmitSites(r.Context(), application.SubmitSitesCommand{ApplicationID: r.PathValue("application_id"), ExpectedVersion: body.ExpectedVersion, ActorID: actor, IdempotencyKey: body.IdempotencyKey, CorrelationID: correlationID(r), Sites: sites})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (a *API) HandleRunChecks(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var body checksRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeDecodeError(w, err)
		return
	}
	app, err := a.service.RunChecks(r.Context(), application.RunChecksCommand{ApplicationID: r.PathValue("application_id"), ExpectedVersion: body.ExpectedVersion, ActorID: actor, IdempotencyKey: body.IdempotencyKey, CorrelationID: correlationID(r)})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (a *API) HandleRemediateSite(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var body remediationRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeDecodeError(w, err)
		return
	}
	var replacement *domain.SiteInput
	if body.Replacement != nil {
		value := body.Replacement.domain()
		replacement = &value
	}
	app, err := a.service.RemediateSite(r.Context(), application.RemediateSiteCommand{ApplicationID: r.PathValue("application_id"), SiteID: r.PathValue("site_id"), Action: body.Action, ExpectedVersion: body.ExpectedVersion, ActorID: actor, IdempotencyKey: body.IdempotencyKey, CorrelationID: correlationID(r), Replacement: replacement})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (a *API) HandleTargetedChecks(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var body targetedChecksRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeDecodeError(w, err)
		return
	}
	app, err := a.service.RunTargetedChecks(r.Context(), application.TargetedCheckCommand{ApplicationID: r.PathValue("application_id"), SiteID: r.PathValue("site_id"), ExpectedVersion: body.ExpectedVersion, ActorID: actor, IdempotencyKey: body.IdempotencyKey, CorrelationID: correlationID(r), Rules: body.Rules})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (a *API) HandleRemediateSites(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var body batchRemediationRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeDecodeError(w, err)
		return
	}
	actions := make([]domain.RemediationAction, len(body.Actions))
	for i, action := range body.Actions {
		actions[i] = domain.RemediationAction{SiteID: action.SiteID, Action: action.Action}
		if action.Replacement != nil {
			value := action.Replacement.domain()
			actions[i].Replacement = &value
		}
	}
	app, err := a.service.RemediateSites(r.Context(), application.BatchRemediationCommand{ApplicationID: r.PathValue("application_id"), ExpectedVersion: body.ExpectedVersion, ActorID: actor, IdempotencyKey: body.IdempotencyKey, CorrelationID: correlationID(r), Actions: actions})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func checkBatchFilter(r *http.Request) (application.CheckBatchFilter, error) {
	filter := application.CheckBatchFilter{SiteID: r.URL.Query().Get("site_id"), Rule: domain.RuleCode(r.URL.Query().Get("rule")), Type: r.URL.Query().Get("type")}
	if raw := r.URL.Query().Get("passed"); raw != "" {
		if raw != "true" && raw != "false" {
			return filter, &domain.ValidationError{Violations: []domain.Violation{{Field: "passed", Code: "invalid_passed", Message: "passed 必须为 true 或 false"}}}
		}
		value, _ := strconv.ParseBool(raw)
		filter.Passed = &value
	}
	return filter, nil
}

func (a *API) HandleListCheckBatches(w http.ResponseWriter, r *http.Request) {
	filter, err := checkBatchFilter(r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	batches, err := a.service.ListCheckBatches(r.Context(), r.PathValue("application_id"), filter)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": batches})
}

func (a *API) HandleGetCheckBatch(w http.ResponseWriter, r *http.Request) {
	filter, err := checkBatchFilter(r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	batch, err := a.service.GetCheckBatch(r.Context(), r.PathValue("application_id"), r.PathValue("batch_id"), filter)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}
