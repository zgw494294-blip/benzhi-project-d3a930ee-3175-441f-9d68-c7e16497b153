package repository

import "context"

func (s *Store) Integrity(ctx context.Context) error {
	var result string
	if e := s.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); e != nil {
		return e
	}
	if result != "ok" {
		return domainErrNotFound("SQLite 完整性检查失败")
	}
	return nil
}
