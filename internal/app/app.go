package app

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/redis"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

type App struct {
	cfg     config.Config
	logger  *logger.Logger
	log     *logger.LayerLogger
	sender  *response.Sender
	db      *database.Database
	redis   *redis.Redis
	modules []Module
}

func New(cfg config.Config, appLogger *logger.Logger, sender *response.Sender, db *database.Database, redisClient *redis.Redis, modules []Module) *App {
	return &App{
		cfg:     cfg,
		logger:  appLogger,
		log:     appLogger.Layer("app"),
		sender:  sender,
		db:      db,
		redis:   redisClient,
		modules: modules,
	}
}

func (a *App) Address() string {
	return ":" + a.cfg.App.Port
}

func (a *App) Logger() *logger.Logger {
	return a.logger
}

func (a *App) Close() error {
	if a.redis != nil {
		_ = a.redis.Close()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
	return a.logger.Close()
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	for _, module := range a.modules {
		a.log.Info(nil, "module.register", "module", module.Name())
		module.RegisterRoutes(mux)
	}

	return middleware.Chain(
		mux,
		middleware.RequestContext,
		middleware.Recover(a.logger, a.sender),
		middleware.RequestLogger(a.logger),
	)
}
