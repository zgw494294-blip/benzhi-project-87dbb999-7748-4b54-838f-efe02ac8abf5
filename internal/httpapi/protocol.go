package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cave-sampling-permit/internal/domain"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	if contentType := r.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		return fmt.Errorf("Content-Type 必须为 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("请求 JSON 无效: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type pointRequest struct {
	X float64 `json:"x_mm"`
	Y float64 `json:"y_mm"`
}
type polygonRequest struct {
	Points []pointRequest `json:"points"`
}

func (p pointRequest) domain() domain.Point { return domain.Point{X: p.X, Y: p.Y} }
func (p polygonRequest) domain() domain.Polygon {
	points := make([]domain.Point, len(p.Points))
	for i := range p.Points {
		points[i] = p.Points[i].domain()
	}
	return domain.Polygon{Points: points}
}

type mutationMeta struct {
	ExpectedVersion int64  `json:"expected_version"`
	IdempotencyKey  string `json:"idempotency_key"`
}

func actorID(r *http.Request) string { return strings.TrimSpace(r.Header.Get("X-Actor-ID")) }
func correlationID(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("X-Correlation-ID"))
	if value == "" {
		value = newCorrelationID()
	}
	return value
}
