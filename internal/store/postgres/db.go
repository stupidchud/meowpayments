// Package postgres provides PostgreSQL implementations of the store interfaces.
package postgres

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meowpayments/meowpayments/internal/config"
)

// Connect creates and validates a pgxpool connection.
func Connect(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

// MustConnect calls Connect and panics on error.
func MustConnect(ctx context.Context, cfg config.DBConfig) *pgxpool.Pool {
	pool, err := Connect(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return pool
}

// Migrate runs all pending up migrations from migrationsDir.
func Migrate(migrationsDir, dsn string) error {
	m, err := migrate.New("file://"+migrationsDir, dsn)
	if err != nil {
		return fmt.Errorf("postgres: migrate init: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("postgres: migrate up: %w", err)
	}
	return nil
}
