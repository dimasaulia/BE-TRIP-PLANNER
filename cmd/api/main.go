package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // trip timezones must resolve even on minimal images

	"github.com/open-suite/boilerplate-golang/internal/app"
)

func main() {
	// Every timestamp is UTC: with Local set to UTC, times scanned from
	// Postgres serialize with a `Z` suffix as the API contract requires.
	time.Local = time.UTC

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.Initialize(ctx)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = application.Close()
	}()

	log := application.Logger().Layer("cmd.api")

	application.StartWorkers(ctx)

	server := &http.Server{
		Addr:              application.Address(),
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info(ctx, "server.start", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error(ctx, "server.error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// WebSockets are hijacked connections, so close them explicitly first.
	application.Shutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error(ctx, "server.shutdown.error", err)
		os.Exit(1)
	}

	log.Info(ctx, "server.stop")
}
