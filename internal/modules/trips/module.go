package trips

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/trips/controllers"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
)

type TripModuleImpl struct {
	TripController controllers.TripController
	auth           *middleware.Auth
}

func NewTripModule(tripController controllers.TripController, auth *middleware.Auth) *TripModuleImpl {
	return &TripModuleImpl{
		TripController: tripController,
		auth:           auth,
	}
}

func (m *TripModuleImpl) Name() string {
	return "trips"
}

func (m *TripModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/trips", m.auth.Require(m.TripController.Create))
	mux.HandleFunc("GET /api/v1/trips", m.auth.Require(m.TripController.List))
	mux.HandleFunc("GET /api/v1/trips/{id}", m.auth.Require(m.TripController.Get))
	mux.HandleFunc("PATCH /api/v1/trips/{id}", m.auth.Require(m.TripController.Update))
	mux.HandleFunc("DELETE /api/v1/trips/{id}", m.auth.Require(m.TripController.Delete))
	mux.HandleFunc("GET /api/v1/trips/{id}/members", m.auth.Require(m.TripController.ListMembers))
	mux.HandleFunc("PATCH /api/v1/trips/{id}/members/{userId}", m.auth.Require(m.TripController.UpdateMember))
	mux.HandleFunc("DELETE /api/v1/trips/{id}/members/{userId}", m.auth.Require(m.TripController.RemoveMember))
}
