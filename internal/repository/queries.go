package repository

import (
	"biocuration/internal/domain"
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func iso(t time.Time) string   { return t.UTC().Format(time.RFC3339Nano) }
func parse(s string) time.Time { t, _ := time.Parse(time.RFC3339Nano, s); return t }
func (s *Store) SaveTree(ctx context.Context, t domain.TreeRecord) error {
	_, e := s.db.ExecContext(ctx, `INSERT INTO trees VALUES(?,?,?,?,?,?,?)`, t.TreeID, t.Species, t.LocationDescription, t.ProtectedStatus, t.BaselineVersion, iso(t.CreatedAt), iso(t.UpdatedAt))
	return e
}

func (s *Store) TreeHasFrozenBatch(ctx context.Context, treeID string) (bool, error) {
	var exists int
	e := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM batches WHERE tree_id=? AND status=?)`, treeID, domain.StatusFrozen).Scan(&exists)
	return exists != 0, e
}

func (s *Store) UpdateTree(ctx context.Context, t domain.TreeRecord, expected int) error {
	r, e := s.db.ExecContext(ctx, `UPDATE trees SET species=?,location_description=?,protected_status=?,baseline_version=?,updated_at=? WHERE tree_id=? AND baseline_version=? AND NOT EXISTS(SELECT 1 FROM batches WHERE tree_id=? AND status=?)`, t.Species, t.LocationDescription, t.ProtectedStatus, t.BaselineVersion, iso(t.UpdatedAt), t.TreeID, expected, t.TreeID, domain.StatusFrozen)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n == 1 {
		return nil
	}
	frozen, e := s.TreeHasFrozenBatch(ctx, t.TreeID)
	if e != nil {
		return e
	}
	if frozen {
		return &domain.DomainError{Code: domain.ErrConflict, Message: "树木存在已冻结批次，不能改变证据语义"}
	}
	return &domain.DomainError{Code: domain.ErrVersionMismatch, Message: "树木基线版本冲突"}
}
func (s *Store) Tree(ctx context.Context, id string) (domain.TreeRecord, error) {
	var t domain.TreeRecord
	var p int
	var c, u string
	e := s.db.QueryRowContext(ctx, `SELECT tree_id,species,location_description,protected_status,baseline_version,created_at,updated_at FROM trees WHERE tree_id=?`, id).Scan(&t.TreeID, &t.Species, &t.LocationDescription, &p, &t.BaselineVersion, &c, &u)
	t.ProtectedStatus = p != 0
	t.CreatedAt = parse(c)
	t.UpdatedAt = parse(u)
	if e == sql.ErrNoRows {
		return t, domainErrNotFound("tree 不存在")
	}
	return t, e
}
func domainErrNotFound(msg string) error {
	return &domain.DomainError{Code: domain.ErrNotFound, Message: msg}
}
func (s *Store) SaveBatch(ctx context.Context, b domain.SamplingBatch) error {
	raw, _ := json.Marshal(b.TargetTissues)
	_, e := s.db.ExecContext(ctx, `INSERT INTO batches VALUES(?,?,?,?,?,?,?,?,?)`, b.BatchID, b.TreeID, b.Collector, iso(b.CollectedAt), string(raw), b.TargetQuantity, b.ExpectedVersion, b.Status, iso(b.CreatedAt))
	return e
}
func (s *Store) Batch(ctx context.Context, id string) (domain.SamplingBatch, error) {
	var b domain.SamplingBatch
	var c, created, status string
	var raw string
	e := s.db.QueryRowContext(ctx, `SELECT batch_id,tree_id,collector,collected_at,target_tissues,target_quantity,expected_version,status,created_at FROM batches WHERE batch_id=?`, id).Scan(&b.BatchID, &b.TreeID, &b.Collector, &c, &raw, &b.TargetQuantity, &b.ExpectedVersion, &status, &created)
	if e == sql.ErrNoRows {
		return b, domainErrNotFound("batch 不存在")
	}
	json.Unmarshal([]byte(raw), &b.TargetTissues)
	b.CollectedAt = parse(c)
	b.CreatedAt = parse(created)
	b.Status = domain.BatchStatus(status)
	return b, e
}
func (s *Store) UpdateBatch(ctx context.Context, b domain.SamplingBatch, expected int) error {
	r, e := s.db.ExecContext(ctx, `UPDATE batches SET expected_version=?,status=? WHERE batch_id=? AND expected_version=?`, b.ExpectedVersion, b.Status, b.BatchID, expected)
	if e == nil {
		n, _ := r.RowsAffected()
		if n == 0 {
			return &domain.DomainError{Code: domain.ErrVersionMismatch, Message: "批次版本冲突"}
		}
	}
	return e
}
func (s *Store) SaveInspection(ctx context.Context, i domain.SampleInspection) error {
	_, e := s.db.ExecContext(ctx, `INSERT INTO inspections VALUES(?,?,?,?,?,?,?,?,?)`, i.SampleID, i.BatchID, i.Label, i.Quantity, i.ContainerCondition, i.ChainNotes, i.QualityStatus, i.ReviewNote, iso(i.RecordedAt))
	return e
}

func (s *Store) InspectionBySampleID(ctx context.Context, sampleID string) (*domain.SampleInspection, error) {
	var i domain.SampleInspection
	var at string
	e := s.db.QueryRowContext(ctx, `SELECT sample_id,batch_id,label,quantity,container_condition,chain_notes,quality_status,review_note,recorded_at FROM inspections WHERE sample_id=?`, sampleID).Scan(&i.SampleID, &i.BatchID, &i.Label, &i.Quantity, &i.ContainerCondition, &i.ChainNotes, &i.QualityStatus, &i.ReviewNote, &at)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	i.RecordedAt = parse(at)
	return &i, e
}

func (s *Store) InspectionByLabel(ctx context.Context, batchID, label string) (*domain.SampleInspection, error) {
	var i domain.SampleInspection
	var at string
	e := s.db.QueryRowContext(ctx, `SELECT sample_id,batch_id,label,quantity,container_condition,chain_notes,quality_status,review_note,recorded_at FROM inspections WHERE batch_id=? AND label=? ORDER BY recorded_at DESC LIMIT 1`, batchID, label).Scan(&i.SampleID, &i.BatchID, &i.Label, &i.Quantity, &i.ContainerCondition, &i.ChainNotes, &i.QualityStatus, &i.ReviewNote, &at)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	i.RecordedAt = parse(at)
	return &i, e
}
func (s *Store) Inspection(ctx context.Context, batchID string) (*domain.SampleInspection, error) {
	var i domain.SampleInspection
	var r string
	var at string
	e := s.db.QueryRowContext(ctx, `SELECT sample_id,batch_id,label,quantity,container_condition,chain_notes,quality_status,review_note,recorded_at FROM inspections WHERE batch_id=? ORDER BY recorded_at DESC LIMIT 1`, batchID).Scan(&i.SampleID, &i.BatchID, &i.Label, &i.Quantity, &i.ContainerCondition, &i.ChainNotes, &i.QualityStatus, &i.ReviewNote, &at)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	i.RecordedAt = parse(at)
	_ = r
	return &i, e
}
func (s *Store) SaveTask(ctx context.Context, t domain.ResamplingTask) error {
	var r any
	if t.ResolvedAt != nil {
		r = iso(*t.ResolvedAt)
	}
	_, e := s.db.ExecContext(ctx, `INSERT INTO resampling_tasks VALUES(?,?,?,?,?,?,?)`, t.TaskID, t.BatchID, t.Reason, t.RequiredActions, t.AssignedTo, t.Status, r)
	return e
}
func (s *Store) Tasks(ctx context.Context, batchID string) ([]domain.ResamplingTask, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT task_id,batch_id,reason,required_actions,assigned_to,status,resolved_at FROM resampling_tasks WHERE batch_id=?`, batchID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.ResamplingTask
	for rows.Next() {
		var t domain.ResamplingTask
		var r sql.NullString
		if e = rows.Scan(&t.TaskID, &t.BatchID, &t.Reason, &t.RequiredActions, &t.AssignedTo, &t.Status, &r); e != nil {
			return nil, e
		}
		if r.Valid {
			x := parse(r.String)
			t.ResolvedAt = &x
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) TasksFiltered(ctx context.Context, batchID string, status domain.ResamplingStatus, limit int) ([]domain.ResamplingTask, int, int, error) {
	var open, closed int
	if e := s.db.QueryRowContext(ctx, `SELECT COUNT(CASE WHEN status='open' THEN 1 END),COUNT(CASE WHEN status='closed' THEN 1 END) FROM resampling_tasks WHERE batch_id=?`, batchID).Scan(&open, &closed); e != nil {
		return nil, 0, 0, e
	}
	query := `SELECT task_id,batch_id,reason,required_actions,assigned_to,status,resolved_at FROM resampling_tasks WHERE batch_id=?`
	args := []any{batchID}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	query += ` ORDER BY task_id LIMIT ?`
	args = append(args, limit)
	rows, e := s.db.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, 0, 0, e
	}
	defer rows.Close()
	out := make([]domain.ResamplingTask, 0)
	for rows.Next() {
		var t domain.ResamplingTask
		var resolved sql.NullString
		if e = rows.Scan(&t.TaskID, &t.BatchID, &t.Reason, &t.RequiredActions, &t.AssignedTo, &t.Status, &resolved); e != nil {
			return nil, 0, 0, e
		}
		if resolved.Valid {
			at := parse(resolved.String)
			t.ResolvedAt = &at
		}
		out = append(out, t)
	}
	return out, open, closed, rows.Err()
}
func (s *Store) UpdateTask(ctx context.Context, t domain.ResamplingTask) error {
	var r any
	if t.ResolvedAt != nil {
		r = iso(*t.ResolvedAt)
	}
	_, e := s.db.ExecContext(ctx, `UPDATE resampling_tasks SET status=?,resolved_at=? WHERE task_id=?`, t.Status, r, t.TaskID)
	return e
}
func (s *Store) Credential(ctx context.Context, id string) (domain.SpecimenCredential, error) {
	var c domain.SpecimenCredential
	var at string
	e := s.db.QueryRowContext(ctx, `SELECT credential_id,batch_id,issued_at,issuer,payload_digest,status FROM credentials WHERE credential_id=?`, id).Scan(&c.CredentialID, &c.BatchID, &at, &c.Issuer, &c.PayloadDigest, &c.Status)
	c.IssuedAt = parse(at)
	if e == sql.ErrNoRows {
		return c, domainErrNotFound("凭据不存在")
	}
	return c, e
}
func (s *Store) CredentialForBatch(ctx context.Context, batchID string) (domain.SpecimenCredential, error) {
	var c domain.SpecimenCredential
	var at string
	e := s.db.QueryRowContext(ctx, `SELECT credential_id,batch_id,issued_at,issuer,payload_digest,status FROM credentials WHERE batch_id=?`, batchID).Scan(&c.CredentialID, &c.BatchID, &at, &c.Issuer, &c.PayloadDigest, &c.Status)
	c.IssuedAt = parse(at)
	if e == sql.ErrNoRows {
		return c, domainErrNotFound("批次凭据不存在")
	}
	return c, e
}
func (s *Store) Audit(ctx context.Context, e domain.AuditEvent) error {
	_, x := s.db.ExecContext(ctx, `INSERT INTO audit_events(aggregate_type,aggregate_id,action,request_key,occurred_at,detail) VALUES(?,?,?,?,?,?)`, e.AggregateType, e.AggregateID, e.Action, e.RequestKey, iso(e.OccurredAt), e.Detail)
	return x
}
func (s *Store) Idempotent(ctx context.Context, key string, v any) ([]byte, bool, error) {
	if strings.TrimSpace(key) == "" {
		return nil, false, nil
	}
	var raw []byte
	e := s.db.QueryRowContext(ctx, `SELECT response FROM idempotency WHERE request_key=?`, key).Scan(&raw)
	if e == nil {
		return raw, true, nil
	}
	if e != sql.ErrNoRows {
		return nil, false, e
	}
	raw, e = json.Marshal(v)
	if e != nil {
		return nil, false, e
	}
	_, e = s.db.ExecContext(ctx, `INSERT INTO idempotency VALUES(?,?)`, key, raw)
	return raw, false, e
}
