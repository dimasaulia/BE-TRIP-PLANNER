package controllers

import (
	"net/http"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

const stateCookieName = "oauth_calendar_state"

type CalendarControllerImpl struct {
	CalendarService services.CalendarService
	response        *response.Sender
	cfg             config.Config
	log             *logger.LayerLogger
}

func NewCalendarController(calendarService services.CalendarService, sender *response.Sender, cfg config.Config, appLogger *logger.Logger) CalendarController {
	return &CalendarControllerImpl{
		CalendarService: calendarService,
		response:        sender,
		cfg:             cfg,
		log:             appLogger.Layer("controller.calendar"),
	}
}

func (c *CalendarControllerImpl) stateCookie() httpx.CookieOptions {
	return httpx.CookieOptions{Name: stateCookieName, Secure: c.cfg.Auth.CookieSecure, Path: "/api/v1/calendar"}
}

func (c *CalendarControllerImpl) Connect(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Connect")
	user, _ := requestctx.UserFrom(r.Context())

	start, err := c.CalendarService.StartConnect(r.Context(), user.ID, r.URL.Query().Get("next"))
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	httpx.SetCookie(w, c.stateCookie(), start.StateCookie, time.Now().Add(10*time.Minute))

	end(nil)
	c.response.Redirect(w, r, start.RedirectURL)
}

func (c *CalendarControllerImpl) Callback(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Callback")
	user, _ := requestctx.UserFrom(r.Context())

	httpx.ClearCookie(w, c.stateCookie())

	if denied := r.URL.Query().Get("error"); denied != "" {
		end(nil, "google_error", denied)
		c.response.Redirect(w, r, c.cfg.App.BaseURL+"/?calendar_error="+denied)
		return
	}

	var stateCookie string
	if cookie, err := r.Cookie(stateCookieName); err == nil {
		stateCookie = cookie.Value
	}

	next, err := c.CalendarService.FinishConnect(r.Context(), user.ID, r.URL.Query().Get("code"), r.URL.Query().Get("state"), stateCookie)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Redirect(w, r, c.cfg.App.BaseURL+next)
}

func (c *CalendarControllerImpl) Disconnect(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Disconnect")
	user, _ := requestctx.UserFrom(r.Context())

	if err := c.CalendarService.Disconnect(r.Context(), user.ID); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "calendar.disconnect.success", nil)
}

func (c *CalendarControllerImpl) EnableSync(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "EnableSync")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	status, err := c.CalendarService.EnableSync(r.Context(), user.ID, tripID)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "calendar.sync.enable.success", status)
}

func (c *CalendarControllerImpl) DisableSync(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "DisableSync")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	if err := c.CalendarService.DisableSync(r.Context(), user.ID, tripID, httpx.QueryBool(r, "keep_events")); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "calendar.sync.disable.success", nil)
}

func (c *CalendarControllerImpl) Resync(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Resync")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	status, err := c.CalendarService.Resync(r.Context(), user.ID, tripID)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusAccepted, "calendar.sync.resync.success", status)
}
