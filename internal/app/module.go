package app

import (
	"context"
	"net/http"
)

type Module interface {
	Name() string
	RegisterRoutes(mux *http.ServeMux)
}

// Worker is an optional capability of a module: Run blocks until ctx is
// cancelled and is started in its own goroutine by App.StartWorkers.
type Worker interface {
	Run(ctx context.Context)
}
