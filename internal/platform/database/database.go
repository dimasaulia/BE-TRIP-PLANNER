package database

import (
	"context"

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
