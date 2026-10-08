package uploads

import (
	"context"
	"net/http"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/modules/uploads/controllers"
	"github.com/open-suite/boilerplate-golang/internal/modules/uploads/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
)

const orphanMinAge = 7 * 24 * time.Hour

type UploadModuleImpl struct {
	UploadController controllers.UploadController
	uploadService    services.UploadService
	auth             *middleware.Auth
	limits           *middleware.Limits
	log              *logger.LayerLogger
}

func NewUploadModule(
	uploadController controllers.UploadController,
	uploadService services.UploadService,
	auth *middleware.Auth,
	limits *middleware.Limits,
	appLogger *logger.Logger,
) *UploadModuleImpl {
	return &UploadModuleImpl{
		UploadController: uploadController,
		uploadService:    uploadService,
		auth:             auth,
		limits:           limits,
		log:              appLogger.Layer("module.uploads"),
	}
}

func (m *UploadModuleImpl) Name() string {
	return "uploads"
}

func (m *UploadModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/trips/{id}/uploads", m.limits.Upload(m.auth.Require(m.UploadController.Upload)))
	mux.HandleFunc("GET /api/v1/files/{tripId}/{storedName}", m.auth.Require(m.UploadController.Serve))
}

// Run deletes images no description references any more, once a day.
func (m *UploadModuleImpl) Run(ctx context.Context) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if _, err := m.uploadService.CleanupOrphans(ctx, orphanMinAge); err != nil {
				m.log.Error(ctx, "cleanup.failed", err)
			}
			timer.Reset(24 * time.Hour)
		}
	}
}
