package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

func newCorrelationID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "req-" + hex.EncodeToString(raw[:])
	}
	return "req-" + time.Now().UTC().Format("20060102150405.000000000")
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (a *API) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		correlation := correlationID(r)
		r.Header.Set("X-Correlation-ID", correlation)
		w.Header().Set("X-Correlation-ID", correlation)
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		a.logger.Info("http_request", "method", r.Method, "path", r.URL.Path, "status", recorder.status, "correlation_id", correlation, "duration_ms", time.Since(started).Milliseconds())
	})
}
