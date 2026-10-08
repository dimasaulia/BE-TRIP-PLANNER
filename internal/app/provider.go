package app

import (
	"github.com/open-suite/boilerplate-golang/internal/modules/auth"
	authServices "github.com/open-suite/boilerplate-golang/internal/modules/auth/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/calendar"
	"github.com/open-suite/boilerplate-golang/internal/modules/health"
	"github.com/open-suite/boilerplate-golang/internal/modules/invites"
	"github.com/open-suite/boilerplate-golang/internal/modules/library"
	"github.com/open-suite/boilerplate-golang/internal/modules/realtime"
	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes"
	"github.com/open-suite/boilerplate-golang/internal/modules/schedule"
	"github.com/open-suite/boilerplate-golang/internal/modules/trips"
	"github.com/open-suite/boilerplate-golang/internal/modules/uploads"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
)

var _ Module = (*health.HealthModuleImpl)(nil)
var _ Module = (*releasenotes.ReleaseNoteModuleImpl)(nil)
var _ Module = (*auth.AuthModuleImpl)(nil)
var _ Module = (*trips.TripModuleImpl)(nil)
var _ Module = (*invites.InviteModuleImpl)(nil)
var _ Module = (*library.LibraryModuleImpl)(nil)
var _ Module = (*schedule.ScheduleModuleImpl)(nil)
var _ Module = (*uploads.UploadModuleImpl)(nil)
var _ Module = (*realtime.RealtimeModuleImpl)(nil)
var _ Module = (*calendar.CalendarModuleImpl)(nil)

var _ Worker = (*auth.AuthModuleImpl)(nil)
var _ Worker = (*uploads.UploadModuleImpl)(nil)
var _ Worker = (*calendar.CalendarModuleImpl)(nil)

func ProvideLogger(cfg config.Config) (*logger.Logger, error) {
	return logger.New(logger.Config{
		Level:  cfg.Logger.Level,
		LogDir: cfg.Logger.LogDir,
	})
}

// ProvideSessionResolver lets the auth middleware use the auth service
// without the shared middleware package importing a module.
func ProvideSessionResolver(service authServices.AuthService) middleware.SessionResolver {
	return service
}

func ProvideModules(
	healthModule *health.HealthModuleImpl,
	releaseNoteModule *releasenotes.ReleaseNoteModuleImpl,
	authModule *auth.AuthModuleImpl,
	tripModule *trips.TripModuleImpl,
	inviteModule *invites.InviteModuleImpl,
	libraryModule *library.LibraryModuleImpl,
	scheduleModule *schedule.ScheduleModuleImpl,
	uploadModule *uploads.UploadModuleImpl,
	realtimeModule *realtime.RealtimeModuleImpl,
	calendarModule *calendar.CalendarModuleImpl,
) []Module {
	return []Module{
		healthModule,
		releaseNoteModule,
		authModule,
		tripModule,
		inviteModule,
		libraryModule,
		scheduleModule,
		uploadModule,
		realtimeModule,
		calendarModule,
	}
}
