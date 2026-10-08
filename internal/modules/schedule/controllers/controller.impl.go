package controllers

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/schedule/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/schedule/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

type ScheduleControllerImpl struct {
	ScheduleService services.ScheduleService
	response        *response.Sender
	log             *logger.LayerLogger
}

func NewScheduleController(scheduleService services.ScheduleService, sender *response.Sender, appLogger *logger.Logger) ScheduleController {
	return &ScheduleControllerImpl{
		ScheduleService: scheduleService,
		response:        sender,
		log:             appLogger.Layer("controller.schedule"),
	}
}

func (c *ScheduleControllerImpl) List(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "List")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	query := r.URL.Query()
	result, err := c.ScheduleService.List(r.Context(), user.ID, tripID, query.Get("from"), query.Get("to"))
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "schedule.list.success", result)
}

func (c *ScheduleControllerImpl) Create(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Create")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	var request dto.CreateBlockRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	block, err := c.ScheduleService.Create(r.Context(), user.ID, tripID, request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusCreated, "schedule.create.success", block)
}

func (c *ScheduleControllerImpl) Update(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Update")
	user, _ := requestctx.UserFrom(r.Context())

	blockID, err := httpx.PathUUID(r, "blockId")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	var request dto.UpdateBlockRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	block, err := c.ScheduleService.Update(r.Context(), user.ID, blockID, request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "schedule.update.success", block)
}

func (c *ScheduleControllerImpl) Delete(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Delete")
	user, _ := requestctx.UserFrom(r.Context())

	blockID, err := httpx.PathUUID(r, "blockId")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	if err := c.ScheduleService.Delete(r.Context(), user.ID, blockID); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "schedule.delete.success", nil)
}
