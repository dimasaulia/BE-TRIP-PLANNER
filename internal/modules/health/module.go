package health

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/health/controllers"
)

type HealthModuleImpl struct {
	HealthController controllers.HealthController
}

func NewHealthModule(healthController controllers.HealthController) *HealthModuleImpl {
	return &HealthModuleImpl{
		HealthController: healthController,
	}
}

func (m *HealthModuleImpl) Name() string {
	return "health"
}

func (m *HealthModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health/live", m.HealthController.Live)
	mux.HandleFunc("GET /health/ready", m.HealthController.Ready)
}
