package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
)

type Database struct {
	Pool *pgxpool.Pool
	log  *logger.LayerLogger
}

func New(ctx context.Context, cfg config.Config, appLogger *logger.Logger) (*Database, error) {
	log := appLogger.Layer("platform.database")
	end := log.Start(ctx, "New")

	poolConfig, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		end(err)
		return nil, err
	}

	poolConfig.MaxConns = cfg.Database.MaxOpenConns
	poolConfig.MinConns = cfg.Database.MinConns
	poolConfig.MaxConnLifetime = cfg.Database.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.Database.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		end(err)
		return nil, err
	}

	end(nil)
	return &Database{
		Pool: pool,
		log:  log,
	}, nil
}

func (d *Database) Ping(ctx context.Context) error {
	return d.Pool.Ping(ctx)
}

func (d *Database) Close() error {
	d.Pool.Close()
	return nil
}

type txKey struct{}

// Querier is satisfied by both the pool and a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Q returns the transaction carried by ctx (see InTx) or the pool, so
// repositories run unchanged inside and outside a transaction.
func (d *Database) Q(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return d.Pool
}

// InTx runs fn inside one transaction; repositories called with the context
// passed to fn join it. A nested call reuses the outer transaction.
func (d *Database) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		return err
	}

	return tx.Commit(ctx)
}
