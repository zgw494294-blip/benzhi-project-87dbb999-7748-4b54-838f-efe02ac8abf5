package application

import (
	"context"
	"errors"
	"time"

	"cave-sampling-permit/internal/domain"
)

var ErrNotFound = errors.New("resource not found")

type VersionConflictError struct{ Expected, Actual int64 }

func (e *VersionConflictError) Error() string {
	return "expected_version does not match current aggregate version"
}

type IdempotencyConflictError struct{ Key string }

func (e *IdempotencyConflictError) Error() string {
	return "idempotency_key was already used with another request"
}

type IdempotencyRecord struct {
	Scope, Key, Operation, Fingerprint string
	Response                           []byte
	CreatedAt                          time.Time
}

type AuditEvent struct {
	ApplicationID, EventType, ActorID, CorrelationID string
	OccurredAt                                       time.Time
	Details                                          map[string]any
}

type Repository interface {
	Create(context.Context, *domain.Application, AuditEvent, IdempotencyRecord) error
	Get(context.Context, string) (*domain.Application, error)
	Update(context.Context, *domain.Application, int64, AuditEvent, IdempotencyRecord) error
	GetIdempotency(context.Context, string, string) (*IdempotencyRecord, error)
	GetPermit(context.Context, string) (*domain.SamplingPermit, error)
	Close() error
}

type Clock interface{ Now() time.Time }
type IDGenerator interface{ New(prefix string) string }
