package application

import (
	"biocuration/internal/domain"
	"biocuration/internal/repository"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	Store *repository.Store
	Clock func() time.Time
}

func New(store *repository.Store) *Service { return &Service{Store: store, Clock: time.Now} }
func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}
func (s *Service) CreateTree(ctx context.Context, id, species, location string, protected bool, key string) (domain.TreeRecord, error) {
	if raw, ok, e := s.Store.Idempotent(ctx, key, map[string]string{"treeID": id}); e != nil {
		return domain.TreeRecord{}, e
	} else if ok {
		_ = raw
		return s.Store.Tree(ctx, id)
	}
	t, e := domain.NewTree(id, species, location, protected, s.now())
	if e != nil {
		return t, e
	}
	if e = s.Store.SaveTree(ctx, t); e != nil {
		return t, e
	}
	s.audit(ctx, "tree", id, "created", key, "树木基线建档")
	return t, nil
}

func (s *Service) UpdateTree(ctx context.Context, pathID, treeID, species, location string, protected bool, expected int, key string) (domain.TreeRecord, error) {
	if raw, ok, e := s.Store.Operation(ctx, "tree_update", key); e != nil {
		return domain.TreeRecord{}, e
	} else if ok {
		if oe := repository.OperationError(raw); oe != nil {
			return domain.TreeRecord{}, oe
		}
		var t domain.TreeRecord
		_ = json.Unmarshal(raw.Response, &t)
		return t, nil
	}
	t, e := s.Store.Tree(ctx, pathID)
	if e == nil {
		if frozen, fe := s.Store.TreeHasFrozenBatch(ctx, pathID); fe != nil {
			e = fe
		} else if frozen {
			e = &domain.DomainError{Code: domain.ErrConflict, Message: "树木存在已冻结批次，不能改变证据语义"}
		}
	}
	if e == nil {
		t, _, e = t.UpdateBaseline(treeID, species, location, protected, expected, s.now())
	}
	if e == nil {
		e = s.Store.UpdateTree(ctx, t, expected)
	}
	if key != "" {
		_ = s.Store.WithTx(ctx, func(tx *sql.Tx) error {
			var rawResp []byte
			if e == nil {
				rawResp, _ = json.Marshal(t)
			}
			return s.Store.SaveOperationTx(tx, "tree_update", key, pathID, rawResp, e)
		})
	}
	if e != nil {
		s.audit(ctx, "tree", pathID, "baseline_update_rejected", key, e.Error())
		return t, e
	}
	s.audit(ctx, "tree", pathID, "baseline_updated", key, "树木基线更新")
	return t, nil
}
func (s *Service) CreateBatch(ctx context.Context, treeID, collector string, collected time.Time, tissues []string, quantity int, key string) (domain.SamplingBatch, error) {
	tree, e := s.Store.Tree(ctx, treeID)
	if e != nil {
		return domain.SamplingBatch{}, e
	}
	id := uuid.NewString()
	if key != "" {
		id = "batch-" + key
		if existing, lookup := s.Store.Batch(ctx, id); lookup == nil {
			return existing, nil
		}
	}
	if e = domain.ValidateCollectionTime(collected); e != nil {
		return domain.SamplingBatch{}, e
	}
	b, e := domain.NewBatch(id, tree, collector, collected, tissues, quantity, s.now())
	if e != nil {
		return b, e
	}
	if e = s.Store.SaveBatch(ctx, b); e != nil {
		return b, e
	}
	s.audit(ctx, "batch", b.BatchID, "created", key, "采样批次登记")
	return b, nil
}
func (s *Service) Inspect(ctx context.Context, batchID, sampleID, label string, quantity int, container, notes, key string, expected ...int) (domain.SampleInspection, []domain.ResamplingTask, domain.SamplingBatch, error) {
	if raw, ok, e := s.Store.Operation(ctx, "inspection", key); e != nil {
		return domain.SampleInspection{}, nil, domain.SamplingBatch{}, e
	} else if ok {
		if oe := repository.OperationError(raw); oe != nil {
			return domain.SampleInspection{}, nil, domain.SamplingBatch{}, oe
		}
		var out struct {
			Inspection domain.SampleInspection `json:"inspection"`
			Tasks      []domain.ResamplingTask `json:"tasks"`
			Batch      domain.SamplingBatch    `json:"batch"`
		}
		if json.Unmarshal(raw.Response, &out) == nil {
			return out.Inspection, out.Tasks, out.Batch, nil
		}
	}
	b, e := s.Store.Batch(ctx, batchID)
	if e != nil {
		return domain.SampleInspection{}, nil, b, e
	}
	if len(expected) > 0 && expected[0] > 0 && expected[0] != b.ExpectedVersion {
		return domain.SampleInspection{}, nil, b, &domain.DomainError{Code: domain.ErrVersionMismatch, Message: "expectedVersion 不匹配"}
	}
	if e = domain.ReconcileQuantity(quantity, b.TargetQuantity); e != nil {
		return domain.SampleInspection{}, nil, b, e
	}
	i, e := domain.Inspect(sampleID, batchID, label, quantity, container, notes, s.now())
	if e != nil {
		return i, nil, b, e
	}
	if existing, lookup := s.Store.InspectionBySampleID(ctx, i.SampleID); lookup != nil {
		return i, nil, b, lookup
	} else if existing != nil {
		if key != "" && existing.BatchID == batchID {
			tasks, _ := s.Store.Tasks(ctx, batchID)
			return *existing, tasks, b, nil
		}
		return i, nil, b, &domain.DomainError{Code: domain.ErrConflict, Message: "sampleID 已存在"}
	}
	if existing, lookup := s.Store.InspectionByLabel(ctx, batchID, i.Label); lookup != nil {
		return i, nil, b, lookup
	} else if existing != nil {
		return i, nil, b, &domain.DomainError{Code: domain.ErrConflict, Message: "标签已存在"}
	}
	old := b.ExpectedVersion
	if e = b.MarkInspection(i.QualityStatus); e != nil {
		return i, nil, b, e
	}
	var generatedTask *domain.ResamplingTask
	if i.QualityStatus == domain.QualityFail {
		t, te := domain.NewResampling("task-"+uuid.NewString(), i, "复核员")
		if te != nil {
			return i, nil, b, te
		}
		generatedTask = &t
	}
	if e = s.Store.WithTx(ctx, func(tx *sql.Tx) error {
		if _, x := tx.ExecContext(ctx, `INSERT INTO inspections VALUES(?,?,?,?,?,?,?,?,?)`, i.SampleID, i.BatchID, i.Label, i.Quantity, i.ContainerCondition, i.ChainNotes, i.QualityStatus, i.ReviewNote, i.RecordedAt.UTC().Format(time.RFC3339Nano)); x != nil {
			return x
		}
		res, x := tx.ExecContext(ctx, `UPDATE batches SET expected_version=?,status=? WHERE batch_id=? AND expected_version=?`, b.ExpectedVersion, b.Status, b.BatchID, old)
		if x != nil {
			return x
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return &domain.DomainError{Code: domain.ErrVersionMismatch, Message: "批次版本冲突"}
		}
		if generatedTask != nil {
			if _, x := tx.ExecContext(ctx, `INSERT INTO resampling_tasks VALUES(?,?,?,?,?,?,?)`, generatedTask.TaskID, generatedTask.BatchID, generatedTask.Reason, generatedTask.RequiredActions, generatedTask.AssignedTo, generatedTask.Status, nil); x != nil {
				return x
			}
		}
		return nil
	}); e != nil {
		return i, nil, b, e
	}
	tasks, _ := s.Store.Tasks(ctx, batchID)
	s.audit(ctx, "batch", batchID, "inspection", key, fmt.Sprintf("质检结果 %s", i.QualityStatus))
	if key != "" {
		response, _ := json.Marshal(map[string]any{"inspection": i, "tasks": tasks, "batch": b})
		_ = s.Store.WithTx(ctx, func(tx *sql.Tx) error {
			return s.Store.SaveOperationTx(tx, "inspection", key, batchID, response, nil)
		})
	}
	return i, tasks, b, nil
}
func (s *Service) Resolve(ctx context.Context, batchID, taskID string, version int, key string) (domain.ResamplingTask, domain.SamplingBatch, error) {
	if raw, ok, e := s.Store.Operation(ctx, "resampling_resolve", key); e != nil {
		return domain.ResamplingTask{}, domain.SamplingBatch{}, e
	} else if ok {
		if oe := repository.OperationError(raw); oe != nil {
			return domain.ResamplingTask{}, domain.SamplingBatch{}, oe
		}
		var out struct {
			Task  domain.ResamplingTask `json:"task"`
			Batch domain.SamplingBatch  `json:"batch"`
		}
		if json.Unmarshal(raw.Response, &out) == nil {
			return out.Task, out.Batch, nil
		}
	}
	b, e := s.Store.Batch(ctx, batchID)
	if e != nil {
		return domain.ResamplingTask{}, b, e
	}
	tasks, e := s.Store.Tasks(ctx, batchID)
	if e != nil {
		return domain.ResamplingTask{}, b, e
	}
	var found *domain.ResamplingTask
	for i := range tasks {
		if tasks[i].TaskID == taskID {
			found = &tasks[i]
			break
		}
	}
	if found == nil {
		return domain.ResamplingTask{}, b, &domain.DomainError{Code: domain.ErrNotFound, Message: "补采任务不存在"}
	}
	if found.Status == domain.ResamplingClosed {
		return *found, b, &domain.DomainError{Code: domain.ErrConflict, Message: "补采任务已关闭"}
	}
	if version != b.ExpectedVersion {
		return domain.ResamplingTask{}, b, &domain.DomainError{Code: domain.ErrVersionMismatch, Message: "expectedVersion 不匹配"}
	}
	resolvedAt := s.now()
	if e = found.Resolve(resolvedAt); e != nil {
		return *found, b, e
	}
	if e = s.Store.ResolveTasksTx(ctx, batchID, []string{taskID}, version, resolvedAt); e != nil {
		return *found, b, e
	}
	oldStatus := b.Status
	b, e = s.Store.Batch(ctx, batchID)
	if e != nil {
		return *found, b, e
	}
	s.audit(ctx, "batch", batchID, "resampling_resolved", key, "补采整改关闭")
	s.audit(ctx, "resampling_task", taskID, "resolved", key, "补采任务关闭")
	if oldStatus != b.Status {
		s.audit(ctx, "batch", batchID, "status_changed", key, string(b.Status))
	}
	if key != "" {
		response, _ := json.Marshal(map[string]any{"task": *found, "batch": b})
		_ = s.Store.WithTx(ctx, func(tx *sql.Tx) error {
			return s.Store.SaveOperationTx(tx, "resampling_resolve", key, batchID, response, nil)
		})
	}
	return *found, b, nil
}

func (s *Service) ResolveAll(ctx context.Context, batchID string, taskIDs []string, version int, key string) ([]domain.ResamplingTask, domain.SamplingBatch, error) {
	if raw, ok, e := s.Store.Operation(ctx, "resampling_resolve_all", key); e != nil {
		return nil, domain.SamplingBatch{}, e
	} else if ok {
		if oe := repository.OperationError(raw); oe != nil {
			return nil, domain.SamplingBatch{}, oe
		}
		var out struct {
			Tasks []domain.ResamplingTask `json:"tasks"`
			Batch domain.SamplingBatch    `json:"batch"`
		}
		if json.Unmarshal(raw.Response, &out) == nil {
			return out.Tasks, out.Batch, nil
		}
	}
	b, e := s.Store.Batch(ctx, batchID)
	if e != nil {
		return nil, b, e
	}
	tasks, e := s.Store.Tasks(ctx, batchID)
	if e != nil {
		return nil, b, e
	}
	if len(taskIDs) == 0 {
		return nil, b, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "taskIDs 不能为空"}
	}
	wanted := map[string]bool{}
	for _, id := range taskIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, b, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "taskIDs 不能包含空值"}
		}
		wanted[id] = true
	}
	for id := range wanted {
		found := false
		for _, t := range tasks {
			if t.TaskID == id {
				found = true
				if t.Status != domain.ResamplingOpen {
					return nil, b, &domain.DomainError{Code: domain.ErrConflict, Message: "补采任务状态冲突"}
				}
				break
			}
		}
		if !found {
			return nil, b, &domain.DomainError{Code: domain.ErrNotFound, Message: "补采任务不存在"}
		}
	}
	uniqueIDs := make([]string, 0, len(wanted))
	for id := range wanted {
		uniqueIDs = append(uniqueIDs, id)
	}
	oldStatus := b.Status
	if e = s.Store.ResolveTasksTx(ctx, batchID, uniqueIDs, version, s.now()); e != nil {
		return nil, b, e
	}
	b, _ = s.Store.Batch(ctx, batchID)
	tasks, _ = s.Store.Tasks(ctx, batchID)
	s.audit(ctx, "batch", batchID, "resampling_resolved_all", key, "批量关闭补采任务")
	for _, id := range uniqueIDs {
		s.audit(ctx, "resampling_task", id, "resolved", key, "补采任务关闭")
	}
	if oldStatus != b.Status {
		s.audit(ctx, "batch", batchID, "status_changed", key, string(b.Status))
	}
	if key != "" {
		response, _ := json.Marshal(map[string]any{"tasks": tasks, "batch": b})
		_ = s.Store.WithTx(ctx, func(tx *sql.Tx) error {
			return s.Store.SaveOperationTx(tx, "resampling_resolve_all", key, batchID, response, nil)
		})
	}
	return tasks, b, nil
}
func (s *Service) Freeze(ctx context.Context, batchID, by string, version int, key string) (domain.PreservationFreeze, domain.SpecimenCredential, error) {
	if raw, ok, e := s.Store.Operation(ctx, "freeze", key); e != nil {
		return domain.PreservationFreeze{}, domain.SpecimenCredential{}, e
	} else if ok {
		if oe := repository.OperationError(raw); oe != nil {
			return domain.PreservationFreeze{}, domain.SpecimenCredential{}, oe
		}
		var out struct {
			Freeze     domain.PreservationFreeze `json:"freeze"`
			Credential domain.SpecimenCredential `json:"credential"`
		}
		if json.Unmarshal(raw.Response, &out) == nil {
			return out.Freeze, out.Credential, nil
		}
	}
	if e := domain.ValidateActor(by); e != nil {
		return domain.PreservationFreeze{}, domain.SpecimenCredential{}, e
	}
	b, e := s.Store.Batch(ctx, batchID)
	if e != nil {
		return domain.PreservationFreeze{}, domain.SpecimenCredential{}, e
	}
	reject := func(err error) (domain.PreservationFreeze, domain.SpecimenCredential, error) {
		s.audit(ctx, "batch", batchID, "freeze_rejected", key, err.Error())
		return domain.PreservationFreeze{}, domain.SpecimenCredential{}, err
	}
	if b.Status == domain.StatusFrozen {
		f, fe := s.Store.FreezeByBatch(ctx, batchID)
		c, _ := s.Store.CredentialForBatch(ctx, batchID)
		if fe == nil {
			s.audit(ctx, "batch", batchID, "freeze_rejected", key, "批次已冻结")
			return f, c, &domain.DomainError{Code: domain.ErrConflict, Message: "批次已冻结"}
		}
	}
	if version != b.ExpectedVersion {
		return reject(&domain.DomainError{Code: domain.ErrVersionMismatch, Message: "expectedVersion 不匹配"})
	}
	i, e := s.Store.Inspection(ctx, batchID)
	if e != nil {
		return reject(e)
	}
	tasks, e := s.Store.Tasks(ctx, batchID)
	if e != nil {
		return reject(e)
	}
	tree, e := s.Store.Tree(ctx, b.TreeID)
	if e != nil {
		return reject(e)
	}
	if e = b.FreezeAllowed(tasks, i); e != nil {
		return reject(e)
	}
	now := s.now()
	digest := domain.EvidenceDigest(tree, b, *i, tasks)
	credID, freezeID := "cred-"+uuid.NewString(), "freeze-"+uuid.NewString()
	if key != "" {
		credID = "cred-" + key
		freezeID = "freeze-" + key
	}
	cred := domain.NewCredential(credID, batchID, "城市树木病害采样保全台", digest, now)
	f := domain.PreservationFreeze{FreezeID: freezeID, BatchID: batchID, EvidenceDigest: digest, FrozenBy: by, FrozenAt: now, CredentialID: cred.CredentialID, Snapshot: domain.BuildEvidenceSnapshot(tree, b, *i, tasks)}
	if e = s.Store.FreezeTx(ctx, f, cred); e != nil {
		s.audit(ctx, "batch", batchID, "freeze_rejected", key, e.Error())
		return f, cred, e
	}
	s.audit(ctx, "batch", batchID, "frozen", key, "冻结并签发凭据")
	if key != "" {
		response, _ := json.Marshal(map[string]any{"freeze": f, "credential": cred})
		_ = s.Store.WithTx(ctx, func(tx *sql.Tx) error { return s.Store.SaveOperationTx(tx, "freeze", key, batchID, response, nil) })
	}
	return f, cred, nil
}
func (s *Service) Verify(ctx context.Context, id string) (bool, string, domain.SpecimenCredential, error) {
	c, e := s.Store.Credential(ctx, id)
	if e != nil {
		return false, "", c, e
	}
	fail := func(message string, err error) (bool, string, domain.SpecimenCredential, error) {
		s.audit(ctx, "credential", id, "verified", "", fmt.Sprintf("result=false reason=%s", message))
		return false, message, c, err
	}
	if c.Status != "valid" {
		return fail("凭据状态无效", nil)
	}
	b, e := s.Store.Batch(ctx, c.BatchID)
	if e != nil {
		return fail("关联批次不存在", e)
	}
	tree, e := s.Store.Tree(ctx, b.TreeID)
	if e != nil {
		return fail("关联树木不存在", e)
	}
	i, e := s.Store.Inspection(ctx, b.BatchID)
	if e != nil {
		return fail("质检记录读取失败", e)
	}
	if i == nil {
		return fail("证据缺失：质检记录不存在", nil)
	}
	tasks, e := s.Store.Tasks(ctx, b.BatchID)
	if e != nil {
		return fail("补采任务读取失败", e)
	}
	if b.Status != domain.StatusFrozen {
		return fail("关联批次未冻结", nil)
	}
	f, e := s.Store.FreezeByCredential(ctx, id)
	if e != nil {
		return fail("证据冻结记录缺失", nil)
	}
	current := domain.EvidenceDigest(tree, b, *i, tasks)
	ok, msg := domain.VerifyCredential(c, current)
	if ok && f.EvidenceDigest != current {
		ok = false
		msg = "冻结证据摘要不匹配"
	}
	s.audit(ctx, "credential", id, "verified", "", fmt.Sprintf("result=%s reason=%s", strconv.FormatBool(ok), msg))
	return ok, msg, c, nil
}

func (s *Service) VerifyDetails(ctx context.Context, id string, history bool, from, to *time.Time, limit int) (map[string]any, error) {
	if limit <= 0 || limit > 100 {
		return nil, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "limit 必须在 1 至 100 之间"}
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "时间范围无效"}
	}
	ok, msg, c, e := s.Verify(ctx, id)
	if e != nil {
		return nil, e
	}
	f, e := s.Store.FreezeByCredential(ctx, id)
	if e != nil {
		return nil, e
	}
	b, e := s.Store.Batch(ctx, c.BatchID)
	if e != nil {
		return nil, e
	}
	out := map[string]any{"valid": ok, "message": msg, "credential": c, "freeze": f, "batchStatus": b.Status}
	if history {
		events, e := s.Store.AuditHistory(ctx, id, "verified", from, to, limit)
		if e != nil {
			return nil, e
		}
		stats := VerifyHistory{Events: events}
		for _, ev := range events {
			if strings.Contains(ev.Detail, "result=true") {
				stats.SuccessCount++
			} else {
				stats.FailureCount++
			}
			if stats.LatestVerifiedAt == nil || ev.OccurredAt.After(*stats.LatestVerifiedAt) {
				x := ev.OccurredAt
				stats.LatestVerifiedAt = &x
			}
		}
		out["history"] = stats
	}
	return out, nil
}
func (s *Service) audit(ctx context.Context, typ, id, action, key, detail string) {
	_ = s.Store.Audit(ctx, domain.AuditEvent{AggregateType: typ, AggregateID: id, Action: action, RequestKey: key, OccurredAt: s.now(), Detail: detail})
}
