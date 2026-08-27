package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/domain"
)

type errorEnvelope struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code, Message string
	Fields        []domain.Violation `json:"fields,omitempty"`
	Details       map[string]any     `json:"details,omitempty"`
}

func (e apiError) MarshalJSON() ([]byte, error) {
	type wire struct {
		Code    string             `json:"code"`
		Message string             `json:"message"`
		Fields  []domain.Violation `json:"fields,omitempty"`
		Details map[string]any     `json:"details,omitempty"`
	}
	return jsonMarshal(wire{Code: e.Code, Message: e.Message, Fields: e.Fields, Details: e.Details})
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var validation *domain.ValidationError
	var state *domain.StateError
	var version *application.VersionConflictError
	var idem *application.IdempotencyConflictError
	switch {
	case errors.As(err, &validation):
		writeJSON(w, http.StatusUnprocessableEntity, errorEnvelope{Error: apiError{Code: "validation_failed", Message: "请求未通过业务校验", Fields: validation.Violations}})
	case errors.As(err, &version):
		writeJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "version_conflict", Message: "申请版本已变化", Details: map[string]any{"expected_version": version.Expected, "actual_version": version.Actual}}})
	case errors.As(err, &idem):
		writeJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "idempotency_conflict", Message: "幂等键已用于不同请求", Details: map[string]any{"idempotency_key": idem.Key}}})
	case errors.As(err, &state):
		writeJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "invalid_state", Message: "当前申请状态不允许该操作", Details: map[string]any{"current_state": state.Current, "expected": state.Expected}}})
	case errors.Is(err, application.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "not_found", Message: "资源不存在"}})
	default:
		writeJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "服务处理请求失败"}})
	}
}

func writeDecodeError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "invalid_json", Message: err.Error()}})
}

func requireActor(w http.ResponseWriter, r *http.Request) (string, bool) {
	actor := actorID(r)
	if actor == "" {
		writeJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "actor_required", Message: "必须提供 X-Actor-ID"}})
		return "", false
	}
	return actor, true
}
