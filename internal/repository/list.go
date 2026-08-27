package repository

import (
	"biocuration/internal/domain"
	"context"
	"database/sql"
	"encoding/json"
)

func (s *Store) ListTrees(ctx context.Context, limit int) ([]domain.TreeRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, e := s.db.QueryContext(ctx, `SELECT tree_id,species,location_description,protected_status,baseline_version,created_at,updated_at FROM trees ORDER BY created_at DESC LIMIT ?`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.TreeRecord{}
	for rows.Next() {
		var t domain.TreeRecord
		var p int
		var c, u string
		if e = rows.Scan(&t.TreeID, &t.Species, &t.LocationDescription, &p, &t.BaselineVersion, &c, &u); e != nil {
			return nil, e
		}
		t.ProtectedStatus = p != 0
		t.CreatedAt = parse(c)
		t.UpdatedAt = parse(u)
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Store) ListBatches(ctx context.Context, treeID string, limit int) ([]domain.SamplingBatch, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, e := s.db.QueryContext(ctx, `SELECT batch_id,tree_id,collector,collected_at,target_tissues,target_quantity,expected_version,status,created_at FROM batches WHERE tree_id=? ORDER BY created_at DESC LIMIT ?`, treeID, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.SamplingBatch{}
	for rows.Next() {
		var b domain.SamplingBatch
		var c, raw, created, status string
		if e = rows.Scan(&b.BatchID, &b.TreeID, &b.Collector, &c, &raw, &b.TargetQuantity, &b.ExpectedVersion, &status, &created); e != nil {
			return nil, e
		}
		b.TargetTissues = decodeTissues(raw)
		b.CollectedAt = parse(c)
		b.CreatedAt = parse(created)
		b.Status = domain.BatchStatus(status)
		out = append(out, b)
	}
	return out, rows.Err()
}
func (s *Store) FreezeByCredential(ctx context.Context, id string) (domain.PreservationFreeze, error) {
	var f domain.PreservationFreeze
	var at string
	var snapshot string
	e := s.db.QueryRowContext(ctx, `SELECT f.freeze_id,f.batch_id,f.evidence_digest,f.frozen_by,f.frozen_at,f.credential_id,s.snapshot FROM freezes f LEFT JOIN freeze_snapshots s ON s.freeze_id=f.freeze_id WHERE f.credential_id=?`, id).Scan(&f.FreezeID, &f.BatchID, &f.EvidenceDigest, &f.FrozenBy, &at, &f.CredentialID, &snapshot)
	f.FrozenAt = parse(at)
	if e == sql.ErrNoRows {
		return f, domainErrNotFound("冻结记录不存在")
	}
	if e == nil {
		_ = json.Unmarshal([]byte(snapshot), &f.Snapshot)
	}
	return f, e
}

func (s *Store) FreezeByBatch(ctx context.Context, batchID string) (domain.PreservationFreeze, error) {
	var f domain.PreservationFreeze
	var at, snapshot string
	e := s.db.QueryRowContext(ctx, `SELECT f.freeze_id,f.batch_id,f.evidence_digest,f.frozen_by,f.frozen_at,f.credential_id,s.snapshot FROM freezes f LEFT JOIN freeze_snapshots s ON s.freeze_id=f.freeze_id WHERE f.batch_id=?`, batchID).Scan(&f.FreezeID, &f.BatchID, &f.EvidenceDigest, &f.FrozenBy, &at, &f.CredentialID, &snapshot)
	if e == sql.ErrNoRows {
		return f, domainErrNotFound("冻结记录不存在")
	}
	if e == nil {
		f.FrozenAt = parse(at)
		_ = json.Unmarshal([]byte(snapshot), &f.Snapshot)
	}
	return f, e
}
