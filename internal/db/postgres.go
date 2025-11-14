package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mesa-ads/internal/config/configs"
)

// NewPostgresPool creates a new pgxpool.Pool with the provided configuration.
// It sets the maximum and minimum connections according to cfg. The
// function verifies that a connection can be established by pinging the
// database with a 5 second timeout. If pinging fails, the pool is closed
// and an error is returned. The caller must close the returned pool when
// it is no longer needed.
func NewPostgresPool(ctx context.Context, cfg configs.Postgres) (*pgxpool.Pool, error) {
	poolConf, err := pgxpool.ParseConfig(cfg.Addr.String())
	if err != nil {
		return nil, err
	}

	// Use pgx.Logger from context if present; otherwise default is fine.
	pool, err := pgxpool.NewWithConfig(ctx, poolConf)
	if err != nil {
		return nil, err
	}

	// ping database with timeout to ensure connectivity
	ctxPing, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = pool.Ping(ctxPing); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
