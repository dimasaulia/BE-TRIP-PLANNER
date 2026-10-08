package controllers

import (
	"net/http"

	"github.com/coder/websocket"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/realtime"
	"github.com/open-suite/boilerplate-golang/internal/shared/access"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/middleware"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

const maxClientsPerTrip = 50

type RealtimeControllerImpl struct {
	hub      *realtime.Hub
	access   access.Service
	response *response.Sender
	cfg      config.Config
	log      *logger.LayerLogger
}

func NewRealtimeController(hub *realtime.Hub, accessService access.Service, sender *response.Sender, cfg config.Config, appLogger *logger.Logger) RealtimeController {
	return &RealtimeControllerImpl{
		hub:      hub,
		access:   accessService,
		response: sender,
		cfg:      cfg,
		log:      appLogger.Layer("controller.realtime"),
	}
}

// Connect upgrades to a WebSocket after checking Origin, session and membership.
// Viewers may connect; the hub only lets owners and editors send previews.
func (c *RealtimeControllerImpl) Connect(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Connect")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	if !middleware.OriginAllowed(c.cfg.Auth.AllowedOrigins, r.Header.Get("Origin")) {
		err := apperror.Forbidden("origin_not_allowed")
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	membership, err := c.access.Require(r.Context(), tripID, user.ID, access.Viewer)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	if c.hub.Count(tripID) >= maxClientsPerTrip {
		err := apperror.TooManyRequests("too_many_connections")
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	// Origin was verified above against ALLOWED_ORIGINS, so the library's own
	// same-host check is switched off.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		end(err)
		return
	}

	end(nil)

	serveErr := c.hub.Serve(r.Context(), conn, tripID, realtime.User{
		ID:        user.ID,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
	}, string(membership.Role))
	if serveErr != nil {
		c.log.Warn(r.Context(), "serve.ended", "error", serveErr.Error())
	}
}
