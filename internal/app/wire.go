//go:build wireinject
// +build wireinject

package app

import (
	"context"

	"github.com/google/wire"
	"github.com/open-suite/boilerplate-golang/internal/modules/auth"
	authControllers "github.com/open-suite/boilerplate-golang/internal/modules/auth/controllers"
	authRepositories "github.com/open-suite/boilerplate-golang/internal/modules/auth/repositories"
	authServices "github.com/open-suite/boilerplate-golang/internal/modules/auth/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/calendar"
	calendarControllers "github.com/open-suite/boilerplate-golang/internal/modules/calendar/controllers"
	calendarRepositories "github.com/open-suite/boilerplate-golang/internal/modules/calendar/repositories"
	calendarServices "github.com/open-suite/boilerplate-golang/internal/modules/calendar/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/health"
	"github.com/open-suite/boilerplate-golang/internal/modules/health/controllers"
	"github.com/open-suite/boilerplate-golang/internal/modules/health/repositories"
	"github.com/open-suite/boilerplate-golang/internal/modules/health/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/invites"
	inviteControllers "github.com/open-suite/boilerplate-golang/internal/modules/invites/controllers"
	inviteRepositories "github.com/open-suite/boilerplate-golang/internal/modules/invites/repositories"
	inviteServices "github.com/open-suite/boilerplate-golang/internal/modules/invites/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/library"
	libraryControllers "github.com/open-suite/boilerplate-golang/internal/modules/library/controllers"
	libraryRepositories "github.com/open-suite/boilerplate-golang/internal/modules/library/repositories"
	libraryServices "github.com/open-suite/boilerplate-golang/internal/modules/library/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/realtime"
	realtimeControllers "github.com/open-suite/boilerplate-golang/internal/modules/realtime/controllers"
	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes"
	releaseNoteControllers "github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/controllers"
	releaseNoteRepositories "github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/repositories"
	releaseNoteServices "github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/schedule"
	scheduleControllers "github.com/open-suite/boilerplate-golang/internal/modules/schedule/controllers"
	scheduleRepositories "github.com/open-suite/boilerplate-golang/internal/modules/schedule/repositories"
	scheduleServices "github.com/open-suite/boilerplate-golang/internal/modules/schedule/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/trips"
	tripControllers "github.com/open-suite/boilerplate-golang/internal/modules/trips/controllers"
	tripRepositories "github.com/open-suite/boilerplate-golang/internal/modules/trips/repositories"
	tripServices "github.com/open-suite/boilerplate-golang/internal/modules/trips/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/uploads"
	uploadControllers "github.com/open-suite/boilerplate-golang/internal/modules/uploads/controllers"
	uploadServices "github.com/open-suite/boilerplate-golang/internal/modules/uploads/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/crypto"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/google"
	"github.com/open-suite/boilerplate-golang/internal/platform/i18n"
	platformRealtime "github.com/open-suite/boilerplate-golang/internal/platform/realtime"
	"github.com/open-suite/boilerplate-golang/internal/platform/redis"
	"github.com/open-suite/boilerplate-golang/internal/platform/storage"
	"github.com/open-suite/boilerplate-golang/internal/shared/access"
	"github.com/open-suite/boilerplate-golang/internal/shared/attachments"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
	"github.com/open-suite/boilerplate-golang/internal/shared/syncqueue"
)

func Initialize(ctx context.Context) (*App, error) {
	wire.Build(
		config.Load,
		ProvideLogger,
		database.New,
		redis.New,
		i18n.NewTranslator,
		response.NewSender,

		crypto.New,
		google.New,
		storage.New,
		platformRealtime.NewHub,
		wire.Bind(new(platformRealtime.Publisher), new(*platformRealtime.Hub)),
		access.NewService,
		attachments.NewService,
		syncqueue.NewQueue,
		ProvideSessionResolver,
		middleware.NewAuth,
		middleware.NewLimits,

		repositories.NewHealthRepository,
		services.NewHealthService,
		controllers.NewHealthController,
		health.NewHealthModule,

		releaseNoteRepositories.NewReleaseNoteRepository,
		releaseNoteServices.NewReleaseNoteService,
		releaseNoteControllers.NewReleaseNoteController,
		releasenotes.NewReleaseNoteModule,

		authRepositories.NewAuthRepository,
		authServices.NewAuthService,
		authControllers.NewAuthController,
		auth.NewAuthModule,

		tripRepositories.NewTripRepository,
		tripServices.NewTripService,
		tripControllers.NewTripController,
		trips.NewTripModule,

		inviteRepositories.NewInviteRepository,
		inviteServices.NewInviteService,
		inviteControllers.NewInviteController,
		invites.NewInviteModule,

		libraryRepositories.NewLibraryRepository,
		libraryServices.NewLibraryService,
		libraryControllers.NewLibraryController,
		library.NewLibraryModule,

		scheduleRepositories.NewScheduleRepository,
		scheduleServices.NewScheduleService,
		scheduleControllers.NewScheduleController,
		schedule.NewScheduleModule,

		uploadServices.NewUploadService,
		uploadControllers.NewUploadController,
		uploads.NewUploadModule,

		realtimeControllers.NewRealtimeController,
		realtime.NewRealtimeModule,

		calendarRepositories.NewCalendarRepository,
		calendarServices.NewCalendarService,
		calendarServices.NewWorker,
		calendarControllers.NewCalendarController,
		calendar.NewCalendarModule,

		ProvideModules,
		New,
	)

	return nil, nil
}
