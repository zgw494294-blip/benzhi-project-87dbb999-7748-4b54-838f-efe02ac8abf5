package httpapi

import (
	"net/http"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/domain"
)

type reviewRequest struct {
	mutationMeta
	ReviewerID string                `json:"reviewer_id"`
	Decisions  []domain.SiteDecision `json:"decisions"`
}
type issuePermitRequest struct{ mutationMeta }

func (a *API) HandleCompleteReview(w http.ResponseWriter, r *http.Request) {
	headerActor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var body reviewRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeDecodeError(w, err)
		return
	}
	if headerActor != body.ReviewerID {
		writeJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "actor_mismatch", Message: "X-Actor-ID 必须与 reviewer_id 一致"}})
		return
	}
	app, err := a.service.CompleteReview(r.Context(), application.ReviewCommand{ApplicationID: r.PathValue("application_id"), ReviewerID: body.ReviewerID, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, CorrelationID: correlationID(r), Decisions: body.Decisions})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (a *API) HandleIssuePermit(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var body issuePermitRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeDecodeError(w, err)
		return
	}
	app, err := a.service.IssuePermit(r.Context(), application.IssuePermitCommand{ApplicationID: r.PathValue("application_id"), ActorID: actor, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: body.IdempotencyKey, CorrelationID: correlationID(r)})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, app.Permit)
}

func (a *API) HandleVerifyPermit(w http.ResponseWriter, r *http.Request) {
	permit, err := a.service.VerifyPermit(r.Context(), r.PathValue("permit_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, permit)
}
