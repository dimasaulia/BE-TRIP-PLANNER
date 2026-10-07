package services

import (
	"context"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/modules/health/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/health/repositories"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
)

type HealthServiceImpl struct {
	HealthRepository repositories.HealthRepository
	log              *logger.LayerLogger
	startedAt        time.Time
}

func NewHealthService(healthRepository repositories.HealthRepository, appLogger *logger.Logger) HealthService {
	return &HealthServiceImpl{
		HealthRepository: healthRepository,
		log:              appLogger.Layer("service.health"),
		startedAt:        time.Now().UTC(),
	}
}

func (s *HealthServiceImpl) Live(ctx context.Context) (dto.Status, error) {
	end := s.log.Start(ctx, "Live")
	defer end(nil)

	return dto.Status{
		Status:    "live",
		StartedAt: &s.startedAt,
	}, nil
}

func (s *HealthServiceImpl) Ready(ctx context.Context) (dto.Status, error) {
	end := s.log.Start(ctx, "Ready")

	if err := s.HealthRepository.Ping(ctx); err != nil {
		end(err)
		return dto.Status{}, err
	}

	end(nil)
	return dto.Status{
		Status: "ready",
	}, nil
}
