package schedule

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/schedule/controllers"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
)

type ScheduleModuleImpl struct {
	ScheduleController controllers.ScheduleController
	auth               *middleware.Auth
	limits             *middleware.Limits
}

func NewScheduleModule(scheduleController controllers.ScheduleController, auth *middleware.Auth, limits *middleware.Limits) *ScheduleModuleImpl {
	return &ScheduleModuleImpl{
		ScheduleController: scheduleController,
		auth:               auth,
		limits:             limits,
	}
}

func (m *ScheduleModuleImpl) Name() string {
	return "schedule"
}

func (m *ScheduleModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/trips/{id}/schedule", m.auth.Require(m.ScheduleController.List))
	mux.HandleFunc("POST /api/v1/trips/{id}/schedule/blocks", m.auth.Require(m.limits.Schedule(m.ScheduleController.Create)))
	mux.HandleFunc("PATCH /api/v1/schedule/blocks/{blockId}", m.auth.Require(m.limits.Schedule(m.ScheduleController.Update)))
	mux.HandleFunc("DELETE /api/v1/schedule/blocks/{blockId}", m.auth.Require(m.limits.Schedule(m.ScheduleController.Delete)))
}
