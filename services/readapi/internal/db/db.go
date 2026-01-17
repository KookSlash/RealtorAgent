package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sylvain/realtoragent/services/readapi/internal/config"
)

type DB struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*DB, error) {
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN())
	if err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() {
	if d.pool != nil {
		d.pool.Close()
	}
}

func (d *DB) Ping(ctx context.Context) error {
	if d.pool == nil {
		return errors.New("db pool not initialized")
	}
	return d.pool.Ping(ctx)
}

func (d *DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return d.pool.QueryRow(ctx, sql, args...)
}

func (d *DB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return d.pool.Query(ctx, sql, args...)
}

func (d *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return d.pool.Exec(ctx, sql, args...)
}
