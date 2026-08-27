package repository

import (
	"biocuration/internal/domain"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func (s *Store) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		_ = tx.Rollback()
		return e
	}
	return tx.Commit()
}

type OperationResult struct {
	Response     []byte
	ErrorCode    domain.ErrorCode
	ErrorMessage string
}

func (s *Store) Operation(ctx context.Context, operation, key string) (OperationResult, bool, error) {
	if key == "" {
		return OperationResult{}, false, nil
	}
	var r OperationResult
	var code, msg string
	e := s.db.QueryRowContext(ctx, `SELECT response,error_code,error_message FROM operation_results WHERE operation=? AND request_key=?`, operation, key).Scan(&r.Response, &code, &msg)
	if e == sql.ErrNoRows {
		return OperationResult{}, false, nil
	}
	if e != nil {
		return OperationResult{}, false, e
	}
	r.ErrorCode, r.ErrorMessage = domain.ErrorCode(code), msg
	return r, true, nil
}

func (s *Store) SaveOperationTx(tx *sql.Tx, operation, key, aggregate string, response []byte, err error) error {
	code, msg := "", ""
	if err != nil {
		msg = err.Error()
		if d, ok := err.(*domain.DomainError); ok {
			code = string(d.Code)
		} else {
			code = "internal_error"
		}
	}
	_, e := tx.Exec(`INSERT INTO operation_results(operation,request_key,aggregate_id,response,error_code,error_message) VALUES(?,?,?,?,?,?)`, operation, key, aggregate, response, code, msg)
	return e
}

func OperationError(r OperationResult) error {
	if r.ErrorCode == "" {
		return nil
	}
	return &domain.DomainError{Code: r.ErrorCode, Message: r.ErrorMessage}
}

func (s *Store) FreezeTx(ctx context.Context, f domain.PreservationFreeze, c domain.SpecimenCredential) error {
	snapshot, e := json.Marshal(f.Snapshot)
	if e != nil {
		return e
	}
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, e = tx.ExecContext(ctx, `INSERT INTO freezes VALUES(?,?,?,?,?,?)`, f.FreezeID, f.BatchID, f.EvidenceDigest, f.FrozenBy, iso(f.FrozenAt), f.CredentialID); e != nil {
			return &domain.DomainError{Code: domain.ErrConflict, Message: fmt.Sprintf("冻结记录冲突: %v", e)}
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO credentials VALUES(?,?,?,?,?,?)`, c.CredentialID, c.BatchID, iso(c.IssuedAt), c.Issuer, c.PayloadDigest, c.Status); e != nil {
			return &domain.DomainError{Code: domain.ErrConflict, Message: fmt.Sprintf("凭据冲突: %v", e)}
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO freeze_snapshots(freeze_id,snapshot) VALUES(?,?)`, f.FreezeID, string(snapshot)); e != nil {
			return e
		}
		res, e := tx.ExecContext(ctx, `UPDATE batches SET status=?,expected_version=expected_version+1 WHERE batch_id=? AND status<>?`, domain.StatusFrozen, f.BatchID, domain.StatusFrozen)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return &domain.DomainError{Code: domain.ErrConflict, Message: "批次已冻结"}
		}
		return nil
	})
}

// SaveFreeze 保留现有仓储入口，并使用不可变快照事务实现。
func (s *Store) SaveFreeze(ctx context.Context, f domain.PreservationFreeze, c domain.SpecimenCredential) error {
	return s.FreezeTx(ctx, f, c)
}

func (s *Store) ResolveTasksTx(ctx context.Context, batchID string, taskIDs []string, expected int, now time.Time) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		for _, id := range taskIDs {
			res, e := tx.ExecContext(ctx, `UPDATE resampling_tasks SET status='closed',resolved_at=? WHERE task_id=? AND batch_id=? AND status='open'`, iso(now), id, batchID)
			if e != nil {
				return e
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return &domain.DomainError{Code: domain.ErrConflict, Message: "补采任务状态冲突"}
			}
		}
		// 每次关闭任务都推进批次版本；只有不存在其他 open 任务时才进入 ready_to_freeze。
		res, e := tx.ExecContext(ctx, `UPDATE batches SET status=CASE WHEN NOT EXISTS(SELECT 1 FROM resampling_tasks WHERE batch_id=? AND status='open') THEN 'ready_to_freeze' ELSE status END,expected_version=expected_version+1 WHERE batch_id=? AND expected_version=? AND status='needs_resampling'`, batchID, batchID, expected)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return &domain.DomainError{Code: domain.ErrVersionMismatch, Message: "批次版本冲突"}
		}
		return nil
	})
}
