package controllers

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/trips/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/trips/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

type TripControllerImpl struct {
	TripService services.TripService
	response    *response.Sender
	log         *logger.LayerLogger
}

func NewTripController(tripService services.TripService, sender *response.Sender, appLogger *logger.Logger) TripController {
	return &TripControllerImpl{
		TripService: tripService,
		response:    sender,
		log:         appLogger.Layer("controller.trips"),
	}
}

func (c *TripControllerImpl) Create(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Create")
	user, _ := requestctx.UserFrom(r.Context())

	var request dto.CreateTripRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	trip, err := c.TripService.Create(r.Context(), user.ID, request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusCreated, "trips.create.success", trip)
}

func (c *TripControllerImpl) List(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "List")
	user, _ := requestctx.UserFrom(r.Context())

	trips, err := c.TripService.List(r.Context(), user.ID)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "trips.list.success", map[string]any{"items": trips})
}

func (c *TripControllerImpl) Get(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Get")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	trip, err := c.TripService.Get(r.Context(), user.ID, tripID)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "trips.get.success", trip)
}

func (c *TripControllerImpl) Update(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Update")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	var request dto.UpdateTripRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	trip, err := c.TripService.Update(r.Context(), user.ID, tripID, request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "trips.update.success", trip)
}

func (c *TripControllerImpl) Delete(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Delete")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	if err := c.TripService.Delete(r.Context(), user.ID, tripID); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "trips.delete.success", nil)
}

func (c *TripControllerImpl) ListMembers(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "ListMembers")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	members, err := c.TripService.ListMembers(r.Context(), user.ID, tripID)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "members.list.success", map[string]any{"items": members})
}

func (c *TripControllerImpl) UpdateMember(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "UpdateMember")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}
	targetID, err := httpx.PathUUID(r, "userId")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	var request dto.UpdateMemberRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	member, err := c.TripService.UpdateMemberRole(r.Context(), user.ID, tripID, targetID, request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "members.update.success", member)
}

func (c *TripControllerImpl) RemoveMember(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "RemoveMember")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}
	targetID, err := httpx.PathUUID(r, "userId")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	if err := c.TripService.RemoveMember(r.Context(), user.ID, tripID, targetID); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "members.remove.success", nil)
}
