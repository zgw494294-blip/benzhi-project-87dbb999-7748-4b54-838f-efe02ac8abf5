package httpapi

import (
	"net/http"

	"cave-sampling-permit/internal/application"
)

type createApplicationRequest struct {
	CaveName          string `json:"cave_name"`
	SegmentName       string `json:"segment_name"`
	ResearchPurpose   string `json:"research_purpose"`
	ApplicantID       string `json:"applicant_id"`
	ProtectionOwnerID string `json:"protection_owner_id"`
	IdempotencyKey    string `json:"idempotency_key"`
}

func (a *API) HandleCreateApplication(w http.ResponseWriter, r *http.Request) {
	var body createApplicationRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeDecodeError(w, err)
		return
	}
	app, err := a.service.CreateApplication(r.Context(), application.CreateApplicationCommand{CaveName: body.CaveName, SegmentName: body.SegmentName, ResearchPurpose: body.ResearchPurpose, ApplicantID: body.ApplicantID, ProtectionOwnerID: body.ProtectionOwnerID, IdempotencyKey: body.IdempotencyKey, CorrelationID: correlationID(r)})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/sampling-applications/"+app.ID)
	writeJSON(w, http.StatusCreated, app)
}

func (a *API) HandleGetApplication(w http.ResponseWriter, r *http.Request) {
	app, err := a.service.GetApplication(r.Context(), r.PathValue("application_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}
