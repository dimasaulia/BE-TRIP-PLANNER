package realtime

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/realtime/controllers"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
)

type RealtimeModuleImpl struct {
	RealtimeController controllers.RealtimeController
	auth               *middleware.Auth
}

func NewRealtimeModule(realtimeController controllers.RealtimeController, auth *middleware.Auth) *RealtimeModuleImpl {
	return &RealtimeModuleImpl{
		RealtimeController: realtimeController,
		auth:               auth,
	}
}

func (m *RealtimeModuleImpl) Name() string {
	return "realtime"
}

func (m *RealtimeModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/trips/{id}/ws", m.auth.Require(m.RealtimeController.Connect))
}
