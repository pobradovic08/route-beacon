package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDatabaseURL = "postgresql://rib:rib@localhost:5432/rib"

// DB wraps a pgx connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB creates a new database connection pool. If databaseURL is empty, the
// default local development URL is used.
func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Close shuts down the connection pool.
func (db *DB) Close() {
	db.Pool.Close()
}
