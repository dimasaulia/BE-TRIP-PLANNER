package services

import (
	"context"

	"github.com/open-suite/boilerplate-golang/internal/modules/health/dto"
)

type HealthService interface {
	Live(ctx context.Context) (dto.Status, error)
	Ready(ctx context.Context) (dto.Status, error)
}
