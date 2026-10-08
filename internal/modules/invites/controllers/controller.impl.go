package controllers

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/invites/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/invites/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

type InviteControllerImpl struct {
	InviteService services.InviteService
	response      *response.Sender
	log           *logger.LayerLogger
}

func NewInviteController(inviteService services.InviteService, sender *response.Sender, appLogger *logger.Logger) InviteController {
	return &InviteControllerImpl{
		InviteService: inviteService,
		response:      sender,
		log:           appLogger.Layer("controller.invites"),
	}
}

func (c *InviteControllerImpl) Create(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Create")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	var request dto.CreateInviteRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	invite, err := c.InviteService.Create(r.Context(), user.ID, tripID, request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusCreated, "invites.create.success", invite)
}

func (c *InviteControllerImpl) List(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "List")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	invites, err := c.InviteService.List(r.Context(), user.ID, tripID)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "invites.list.success", map[string]any{"items": invites})
}

func (c *InviteControllerImpl) Revoke(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Revoke")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}
	inviteID, err := httpx.PathUUID(r, "inviteId")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	if err := c.InviteService.Revoke(r.Context(), user.ID, tripID, inviteID); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "invites.revoke.success", nil)
}

func (c *InviteControllerImpl) Preview(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Preview")

	preview, err := c.InviteService.Preview(r.Context(), r.PathValue("token"))
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "invites.preview.success", preview)
}

func (c *InviteControllerImpl) Accept(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Accept")
	user, _ := requestctx.UserFrom(r.Context())

	result, err := c.InviteService.Accept(r.Context(), user.ID, r.PathValue("token"))
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "invites.accept.success", result)
}
