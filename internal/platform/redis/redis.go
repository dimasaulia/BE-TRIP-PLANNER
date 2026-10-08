package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
)

// Redis is optional: with REDIS_ADDR empty, Client is nil and Ping/Close are no-ops.
type Redis struct {
	Client *goredis.Client
	log    *logger.LayerLogger
}

func New(ctx context.Context, cfg config.Config, appLogger *logger.Logger) (*Redis, error) {
	log := appLogger.Layer("platform.redis")
	end := log.Start(ctx, "New")

	if cfg.Redis.Addr == "" {
		end(nil, "enabled", false)
		return &Redis{log: log}, nil
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Username: cfg.Redis.Username,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	end(nil)
	return &Redis{
		Client: client,
		log:    log,
	}, nil
}

func (r *Redis) Enabled() bool {
	return r != nil && r.Client != nil
}

func (r *Redis) Ping(ctx context.Context) error {
	if !r.Enabled() {
		return nil
	}
	return r.Client.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	if !r.Enabled() {
		return nil
	}
	return r.Client.Close()
}
