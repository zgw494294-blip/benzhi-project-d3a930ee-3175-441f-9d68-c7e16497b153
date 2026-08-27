package repository

import (
	"biocuration/internal/domain"
	"context"
	"time"
)

func (s *Store) AuditCount(ctx context.Context) (int, error) {
	var n int
	e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&n)
	return n, e
}
func (s *Store) RecentAudit(ctx context.Context, limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id,aggregate_type,aggregate_id,action,request_key,occurred_at,detail FROM audit_events ORDER BY id DESC LIMIT ?", limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.AuditEvent
	for rows.Next() {
		var a domain.AuditEvent
		var at string
		if e = rows.Scan(&a.ID, &a.AggregateType, &a.AggregateID, &a.Action, &a.RequestKey, &at, &a.Detail); e != nil {
			return nil, e
		}
		a.OccurredAt = parse(at)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) AuditHistory(ctx context.Context, aggregateID, action string, from, to *time.Time, limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := `SELECT id,aggregate_type,aggregate_id,action,request_key,occurred_at,detail FROM audit_events WHERE aggregate_id=? AND action=?`
	args := []any{aggregateID, action}
	if from != nil {
		q += ` AND occurred_at>=?`
		args = append(args, iso(*from))
	}
	if to != nil {
		q += ` AND occurred_at<=?`
		args = append(args, iso(*to))
	}
	q += ` ORDER BY occurred_at DESC,id DESC LIMIT ?`
	args = append(args, limit)
	rows, e := s.db.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var a domain.AuditEvent
		var at string
		if e = rows.Scan(&a.ID, &a.AggregateType, &a.AggregateID, &a.Action, &a.RequestKey, &at, &a.Detail); e != nil {
			return nil, e
		}
		a.OccurredAt = parse(at)
		out = append(out, a)
	}
	return out, rows.Err()
}
