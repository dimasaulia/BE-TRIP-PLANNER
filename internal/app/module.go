package app

import "net/http"

type Module interface {
	Name() string
	RegisterRoutes(mux *http.ServeMux)
}
