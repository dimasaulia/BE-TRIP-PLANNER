package controllers

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/health/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

type HealthControllerImpl struct {
	HealthService services.HealthService
	response      *response.Sender
	log           *logger.LayerLogger
}

func NewHealthController(healthService services.HealthService, sender *response.Sender, appLogger *logger.Logger) HealthController {
	return &HealthControllerImpl{
		HealthService: healthService,
		response:      sender,
		log:           appLogger.Layer("controller.health"),
	}
}

func (c *HealthControllerImpl) Live(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Live")

	status, err := c.HealthService.Live(r.Context())
	if err != nil {
		end(err)
		c.response.Error(w, r, http.StatusInternalServerError, "error.internal", nil)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "health.live", status)
}

func (c *HealthControllerImpl) Ready(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Ready")

	status, err := c.HealthService.Ready(r.Context())
	if err != nil {
		end(err)
		c.response.Error(w, r, http.StatusInternalServerError, "error.internal", nil)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "health.ready", status)
}
