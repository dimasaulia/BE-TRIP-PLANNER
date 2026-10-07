package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
)

type Redis struct {
	Client *goredis.Client
	log    *logger.LayerLogger
}

func New(ctx context.Context, cfg config.Config, appLogger *logger.Logger) (*Redis, error) {
	log := appLogger.Layer("platform.redis")
	end := log.Start(ctx, "New")

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

func (r *Redis) Ping(ctx context.Context) error {
	return r.Client.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	return r.Client.Close()
}
