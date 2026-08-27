package application

import "time"

func (s *Service) makeIdempotency(scope, key, operation, fingerprint string, now time.Time) IdempotencyRecord {
	return IdempotencyRecord{Scope: scope, Key: key, Operation: operation, Fingerprint: fingerprint, CreatedAt: now.UTC()}
}
