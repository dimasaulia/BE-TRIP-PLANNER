package library

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/library/controllers"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
)

type LibraryModuleImpl struct {
	LibraryController controllers.LibraryController
	auth              *middleware.Auth
}

func NewLibraryModule(libraryController controllers.LibraryController, auth *middleware.Auth) *LibraryModuleImpl {
	return &LibraryModuleImpl{
		LibraryController: libraryController,
		auth:              auth,
	}
}

func (m *LibraryModuleImpl) Name() string {
	return "library"
}

func (m *LibraryModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/trips/{id}/library", m.auth.Require(m.LibraryController.List))
	mux.HandleFunc("POST /api/v1/trips/{id}/library", m.auth.Require(m.LibraryController.Create))
	mux.HandleFunc("POST /api/v1/trips/{id}/library/export", m.auth.Require(m.LibraryController.Export))
	mux.HandleFunc("GET /api/v1/library/{itemId}", m.auth.Require(m.LibraryController.Get))
	mux.HandleFunc("PATCH /api/v1/library/{itemId}", m.auth.Require(m.LibraryController.Update))
	mux.HandleFunc("DELETE /api/v1/library/{itemId}", m.auth.Require(m.LibraryController.Delete))
}
