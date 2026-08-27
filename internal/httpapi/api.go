package httpapi

import (
	"log/slog"
	"net/http"

	"cave-sampling-permit/internal/application"
)

const maxRequestBytes int64 = 1 << 20

type API struct {
	service *application.Service
	logger  *slog.Logger
	mux     *http.ServeMux
}

func New(service *application.Service, logger *slog.Logger) *API {
	if logger == nil {
		logger = slog.Default()
	}
	a := &API{service: service, logger: logger, mux: http.NewServeMux()}
	a.routes()
	return a
}

func (a *API) Handler() http.Handler { return a.requestLog(a.mux) }

func (a *API) routes() {
	a.mux.HandleFunc("GET /healthz", a.HandleHealth)
	a.mux.HandleFunc("POST /api/v1/sampling-applications", a.HandleCreateApplication)
	a.mux.HandleFunc("GET /api/v1/sampling-applications/{application_id}", a.HandleGetApplication)
	a.mux.HandleFunc("POST /api/v1/sampling-applications/{application_id}/baseline:freeze", a.HandleFreezeBaseline)
	a.mux.HandleFunc("POST /api/v1/sampling-applications/{application_id}/sites:submit", a.HandleSubmitSites)
	a.mux.HandleFunc("POST /api/v1/sampling-applications/{application_id}/checks:run", a.HandleRunChecks)
	a.mux.HandleFunc("GET /api/v1/sampling-applications/{application_id}/check-batches", a.HandleListCheckBatches)
	a.mux.HandleFunc("GET /api/v1/sampling-applications/{application_id}/check-batches/{batch_id}", a.HandleGetCheckBatch)
	a.mux.HandleFunc("POST /api/v1/sampling-applications/{application_id}/sites:remediate", a.HandleRemediateSites)
	a.mux.HandleFunc("POST /api/v1/sampling-applications/{application_id}/sites/{site_id}/remediate", a.HandleRemediateSite)
	a.mux.HandleFunc("POST /api/v1/sampling-applications/{application_id}/sites/{site_id}/checks:run", a.HandleTargetedChecks)
	a.mux.HandleFunc("POST /api/v1/sampling-applications/{application_id}/reviews", a.HandleCompleteReview)
	a.mux.HandleFunc("POST /api/v1/sampling-applications/{application_id}/permit:issue", a.HandleIssuePermit)
	a.mux.HandleFunc("GET /api/v1/permits/{permit_id}", a.HandleVerifyPermit)
}

func (a *API) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
