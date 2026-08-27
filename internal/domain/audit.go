package domain

import "time"

type AuditEvent struct {
	ID                                             int64
	AggregateType, AggregateID, Action, RequestKey string
	OccurredAt                                     time.Time
	Detail                                         string
}
