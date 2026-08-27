package application

import (
	"biocuration/internal/domain"
	"context"
)

type SystemReport struct {
	Trees   int  `json:"trees"`
	Audits  int  `json:"audits"`
	Healthy bool `json:"healthy"`
}

func (s *Service) Report(ctx context.Context) (SystemReport, error) {
	trees, e := s.Store.ListTrees(ctx, 1<<10)
	if e != nil {
		return SystemReport{}, e
	}
	audits, e := s.Store.AuditCount(ctx)
	if e != nil {
		return SystemReport{}, e
	}
	return SystemReport{Trees: len(trees), Audits: audits, Healthy: true}, nil
}
func (s *Service) ValidatePolicy(ctx context.Context, treeID string, tissues []string, quantity int) error {
	tree, e := s.Store.Tree(ctx, treeID)
	if e != nil {
		return e
	}
	if e = domain.PolicyFor(tree).Allows(tissues, quantity); e != nil {
		return e
	}
	return domain.ValidateCollectionTime(s.now())
}
