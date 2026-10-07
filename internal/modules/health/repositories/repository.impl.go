package repositories

import (
	"context"

	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/redis"
)

type HealthRepositoryImpl struct {
	db    *database.Database
	redis *redis.Redis
}

func NewHealthRepository(db *database.Database, redisClient *redis.Redis) HealthRepository {
	return &HealthRepositoryImpl{
		db:    db,
		redis: redisClient,
	}
}

func (r *HealthRepositoryImpl) Ping(ctx context.Context) error {
	if err := r.db.Ping(ctx); err != nil {
		return err
	}

	return r.redis.Ping(ctx)
}
