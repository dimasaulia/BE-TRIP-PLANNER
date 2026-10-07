package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/app"
)

func main() {
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

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error(ctx, "server.shutdown.error", err)
		os.Exit(1)
	}

	log.Info(ctx, "server.stop")
}
