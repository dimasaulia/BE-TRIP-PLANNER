package invites

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/invites/controllers"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
)

type InviteModuleImpl struct {
	InviteController controllers.InviteController
	auth             *middleware.Auth
	limits           *middleware.Limits
}

func NewInviteModule(inviteController controllers.InviteController, auth *middleware.Auth, limits *middleware.Limits) *InviteModuleImpl {
	return &InviteModuleImpl{
		InviteController: inviteController,
		auth:             auth,
		limits:           limits,
	}
}

func (m *InviteModuleImpl) Name() string {
	return "invites"
}

func (m *InviteModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/trips/{id}/invites", m.auth.Require(m.InviteController.Create))
	mux.HandleFunc("GET /api/v1/trips/{id}/invites", m.auth.Require(m.InviteController.List))
	mux.HandleFunc("DELETE /api/v1/trips/{id}/invites/{inviteId}", m.auth.Require(m.InviteController.Revoke))
	mux.HandleFunc("GET /api/v1/invites/{token}", m.limits.Invite(m.InviteController.Preview))
	mux.HandleFunc("POST /api/v1/invites/{token}/accept", m.limits.Invite(m.auth.Require(m.InviteController.Accept)))
}
