package calendar

import (
	"context"
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/controllers"
	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/services"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
)

type CalendarModuleImpl struct {
	CalendarController controllers.CalendarController
	worker             *services.Worker
	auth               *middleware.Auth
	limits             *middleware.Limits
}

func NewCalendarModule(
	calendarController controllers.CalendarController,
	worker *services.Worker,
	auth *middleware.Auth,
	limits *middleware.Limits,
) *CalendarModuleImpl {
	return &CalendarModuleImpl{
		CalendarController: calendarController,
		worker:             worker,
		auth:               auth,
		limits:             limits,
	}
}

func (m *CalendarModuleImpl) Name() string {
	return "calendar"
}

func (m *CalendarModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/calendar/connect", m.limits.Auth(m.auth.Require(m.CalendarController.Connect)))
	mux.HandleFunc("GET /api/v1/calendar/callback", m.limits.Auth(m.auth.Require(m.CalendarController.Callback)))
	mux.HandleFunc("DELETE /api/v1/calendar/connection", m.auth.Require(m.CalendarController.Disconnect))
	mux.HandleFunc("PUT /api/v1/trips/{id}/calendar-sync", m.auth.Require(m.CalendarController.EnableSync))
	mux.HandleFunc("DELETE /api/v1/trips/{id}/calendar-sync", m.auth.Require(m.CalendarController.DisableSync))
	mux.HandleFunc("POST /api/v1/trips/{id}/calendar-sync/resync", m.auth.Require(m.CalendarController.Resync))
}

// Run starts the sync worker for the lifetime of the app.
func (m *CalendarModuleImpl) Run(ctx context.Context) {
	m.worker.Run(ctx)
}
