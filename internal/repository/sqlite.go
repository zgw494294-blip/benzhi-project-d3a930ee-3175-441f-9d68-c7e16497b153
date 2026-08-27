package repository

import (
	"context"
	"database/sql"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	db.SetMaxOpenConns(1)
	if err = s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) init() error {
	_, err := s.db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`)
	if err != nil {
		return err
	}
	return migrate(s.db)
}
func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Store) DB() *sql.DB                    { return s.db }
