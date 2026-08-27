package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/storage"
)

func TestCreateAndStableValidationError(t *testing.T) {
	repo, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := application.NewService(repo, application.RealClock{}, &application.RandomIDGenerator{})
	server := httptest.NewServer(New(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
	defer server.Close()
	body := map[string]any{"cave_name": "", "segment_name": "段", "research_purpose": "研究", "applicant_id": "a", "protection_owner_id": "o", "idempotency_key": "key"}
	data, _ := json.Marshal(body)
	response, err := http.Post(server.URL+"/api/v1/sampling-applications", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", response.StatusCode)
	}
	var envelope map[string]any
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	errorValue := envelope["error"].(map[string]any)
	if errorValue["code"] != "validation_failed" {
		t.Fatalf("code=%v", errorValue["code"])
	}
}
