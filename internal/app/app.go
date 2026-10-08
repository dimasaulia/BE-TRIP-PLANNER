package app

import (
	"context"
	"net/http"
	"sync"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/realtime"
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
	hub     *realtime.Hub
	modules []Module
	workers sync.WaitGroup
}

func New(cfg config.Config, appLogger *logger.Logger, sender *response.Sender, db *database.Database, redisClient *redis.Redis, hub *realtime.Hub, modules []Module) *App {
	return &App{
		cfg:     cfg,
		logger:  appLogger,
		log:     appLogger.Layer("app"),
		sender:  sender,
		db:      db,
		redis:   redisClient,
		hub:     hub,
		modules: modules,
	}
}

func (a *App) Address() string {
	return ":" + a.cfg.App.Port
}

func (a *App) Logger() *logger.Logger {
	return a.logger
}

// StartWorkers launches every module that is also a Worker (calendar sync,
// session purge, orphan image cleanup). They stop when ctx is cancelled.
func (a *App) StartWorkers(ctx context.Context) {
	for _, module := range a.modules {
		worker, ok := module.(Worker)
		if !ok {
			continue
		}

		a.log.Info(ctx, "worker.register", "module", module.Name())
		a.workers.Add(1)
		go func() {
			defer a.workers.Done()
			worker.Run(ctx)
		}()
	}
}

// Shutdown closes WebSocket connections, which http.Server.Shutdown does not
// wait for, then waits for the workers to finish.
func (a *App) Shutdown() {
	a.hub.Shutdown()
	a.workers.Wait()
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
		middleware.SecurityHeaders(a.cfg.Auth.CookieSecure),
		middleware.Recover(a.logger, a.sender),
		middleware.RequestLogger(a.logger),
		middleware.Origin(a.cfg.Auth.AllowedOrigins, a.sender),
	)
}
