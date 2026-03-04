package core

import (
	"context"
	"database/sql"
)

type Repository interface {
	WithTx(tx *sql.Tx) Repository
}

type BaseRepository struct {
	db *sql.DB
	tx *sql.Tx
}

func NewBaseRepository(db *sql.DB) *BaseRepository {
	return &BaseRepository{db: db}
}

func (r *BaseRepository) DB() *sql.DB {
	return r.db
}

func (r *BaseRepository) Tx() *sql.Tx {
	return r.tx
}

func (r *BaseRepository) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *BaseRepository) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *BaseRepository) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *BaseRepository) WithTx(tx *sql.Tx) Repository {
	return &BaseRepository{
		db: r.db,
		tx: tx,
	}
}
