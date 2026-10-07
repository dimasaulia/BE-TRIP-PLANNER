//go:build wireinject
// +build wireinject

package app

import (
	"context"

	"github.com/google/wire"
	"github.com/open-suite/boilerplate-golang/internal/modules/health"
	"github.com/open-suite/boilerplate-golang/internal/modules/health/controllers"
	"github.com/open-suite/boilerplate-golang/internal/modules/health/repositories"
	"github.com/open-suite/boilerplate-golang/internal/modules/health/services"
	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes"
	releaseNoteControllers "github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/controllers"
	releaseNoteRepositories "github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/repositories"
	releaseNoteServices "github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/i18n"
	"github.com/open-suite/boilerplate-golang/internal/platform/redis"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

func Initialize(ctx context.Context) (*App, error) {
	wire.Build(
		config.Load,
		ProvideLogger,
		database.New,
		redis.New,
		i18n.NewTranslator,
		response.NewSender,
		repositories.NewHealthRepository,
		services.NewHealthService,
		controllers.NewHealthController,
		health.NewHealthModule,
		releaseNoteRepositories.NewReleaseNoteRepository,
		releaseNoteServices.NewReleaseNoteService,
		releaseNoteControllers.NewReleaseNoteController,
		releasenotes.NewReleaseNoteModule,
		ProvideModules,
		New,
	)

	return nil, nil
}
