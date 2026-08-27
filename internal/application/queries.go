package application

import (
	"biocuration/internal/domain"
	"context"
	"time"
)

func (s *Service) GetBatch(ctx context.Context, id string) (domain.SamplingBatch, error) {
	return s.Store.Batch(ctx, id)
}
func (s *Service) GetTree(ctx context.Context, id string) (domain.TreeRecord, error) {
	return s.Store.Tree(ctx, id)
}
func (s *Service) AuditCount(ctx context.Context) (int, error) { return s.Store.AuditCount(ctx) }

type ResamplingReport struct {
	Tasks  []domain.ResamplingTask `json:"tasks"`
	Open   int                     `json:"open"`
	Closed int                     `json:"closed"`
	Batch  domain.SamplingBatch    `json:"batch"`
}

func (s *Service) ListResampling(ctx context.Context, batchID string, status domain.ResamplingStatus, limit int) (ResamplingReport, error) {
	if status != "" && status != domain.ResamplingOpen && status != domain.ResamplingClosed {
		return ResamplingReport{}, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "status 必须为 open 或 closed"}
	}
	if limit <= 0 || limit > 100 {
		return ResamplingReport{}, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "limit 必须在 1 至 100 之间"}
	}
	b, e := s.Store.Batch(ctx, batchID)
	if e != nil {
		return ResamplingReport{}, e
	}
	tasks, open, closed, e := s.Store.TasksFiltered(ctx, batchID, status, limit)
	return ResamplingReport{Tasks: tasks, Open: open, Closed: closed, Batch: b}, e
}

type VerifyHistory struct {
	Events           []domain.AuditEvent `json:"events"`
	SuccessCount     int                 `json:"successCount"`
	FailureCount     int                 `json:"failureCount"`
	LatestVerifiedAt *time.Time          `json:"latestVerifiedAt,omitempty"`
}
