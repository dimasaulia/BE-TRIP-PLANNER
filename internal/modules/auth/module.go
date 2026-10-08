package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/modules/auth/controllers"
	"github.com/open-suite/boilerplate-golang/internal/modules/auth/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
)

type AuthModuleImpl struct {
	AuthController controllers.AuthController
	authService    services.AuthService
	auth           *middleware.Auth
	limits         *middleware.Limits
	cfg            config.Config
	log            *logger.LayerLogger
}

func NewAuthModule(
	authController controllers.AuthController,
	authService services.AuthService,
	auth *middleware.Auth,
	limits *middleware.Limits,
	cfg config.Config,
	appLogger *logger.Logger,
) *AuthModuleImpl {
	return &AuthModuleImpl{
		AuthController: authController,
		authService:    authService,
		auth:           auth,
		limits:         limits,
		cfg:            cfg,
		log:            appLogger.Layer("module.auth"),
	}
}

func (m *AuthModuleImpl) Name() string {
	return "auth"
}

func (m *AuthModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/google/login", m.limits.Auth(m.AuthController.GoogleLogin))
	mux.HandleFunc("GET /api/v1/auth/google/callback", m.limits.Auth(m.AuthController.GoogleCallback))
	mux.HandleFunc("POST /api/v1/auth/logout", m.auth.Require(m.AuthController.Logout))
	mux.HandleFunc("GET /api/v1/me", m.auth.Require(m.AuthController.Me))

	if m.cfg.Auth.DevLoginEnabled {
		m.log.Warn(nil, "dev login is enabled; never use this setting in production")
		mux.HandleFunc("POST /api/v1/auth/dev-login", m.limits.Auth(m.AuthController.DevLogin))
	}
}

// Run purges expired sessions hourly.
func (m *AuthModuleImpl) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			removed, err := m.authService.PurgeExpired(ctx)
			if err != nil {
				m.log.Error(ctx, "purge.failed", err)
				continue
			}
			m.log.Info(ctx, "purge.done", "removed", removed)
		}
	}
}
