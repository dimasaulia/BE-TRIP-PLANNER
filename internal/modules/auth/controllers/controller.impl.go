package controllers

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/auth/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/auth/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

const stateCookieName = "oauth_state"

type AuthControllerImpl struct {
	AuthService services.AuthService
	response    *response.Sender
	cfg         config.Config
	log         *logger.LayerLogger
}

func NewAuthController(authService services.AuthService, sender *response.Sender, cfg config.Config, appLogger *logger.Logger) AuthController {
	return &AuthControllerImpl{
		AuthService: authService,
		response:    sender,
		cfg:         cfg,
		log:         appLogger.Layer("controller.auth"),
	}
}

func (c *AuthControllerImpl) sessionCookie() httpx.CookieOptions {
	return httpx.CookieOptions{Name: c.cfg.Auth.SessionCookieName(), Secure: c.cfg.Auth.CookieSecure}
}

func (c *AuthControllerImpl) stateCookie() httpx.CookieOptions {
	return httpx.CookieOptions{Name: stateCookieName, Secure: c.cfg.Auth.CookieSecure, Path: "/api/v1/auth/google"}
}

func (c *AuthControllerImpl) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "GoogleLogin")

	start, err := c.AuthService.StartLogin(r.Context(), r.URL.Query().Get("next"))
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	httpx.SetCookie(w, c.stateCookie(), start.StateCookie, timeIn(600))

	end(nil)
	c.response.Redirect(w, r, start.RedirectURL)
}

func (c *AuthControllerImpl) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "GoogleCallback")

	// The state cookie is single use whatever the outcome.
	httpx.ClearCookie(w, c.stateCookie())

	if denied := r.URL.Query().Get("error"); denied != "" {
		end(nil, "google_error", denied)
		c.response.Redirect(w, r, c.cfg.App.BaseURL+"/?login_error="+denied)
		return
	}

	var stateCookie string
	if cookie, err := r.Cookie(stateCookieName); err == nil {
		stateCookie = cookie.Value
	}

	result, err := c.AuthService.FinishLogin(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"), stateCookie)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	httpx.SetCookie(w, c.sessionCookie(), result.Token, result.Expires)

	end(nil)
	c.response.Redirect(w, r, c.cfg.App.BaseURL+result.Next)
}

func (c *AuthControllerImpl) DevLogin(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "DevLogin")

	var request dto.DevLoginRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	result, err := c.AuthService.DevLogin(r.Context(), request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	httpx.SetCookie(w, c.sessionCookie(), result.Token, result.Expires)

	end(nil)
	c.response.Success(w, r, http.StatusOK, "auth.login.success", map[string]any{"user": result.User, "expires_at": result.Expires.UTC()})
}

func (c *AuthControllerImpl) Logout(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Logout")

	if cookie, err := r.Cookie(c.sessionCookie().Name); err == nil {
		if err := c.AuthService.Logout(r.Context(), cookie.Value); err != nil {
			end(err)
			c.response.Fail(w, r, err)
			return
		}
	}
	httpx.ClearCookie(w, c.sessionCookie())

	end(nil)
	c.response.Success(w, r, http.StatusOK, "auth.logout.success", nil)
}

func (c *AuthControllerImpl) Me(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Me")

	user, _ := requestctx.UserFrom(r.Context())
	me, err := c.AuthService.Me(r.Context(), user.ID)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "auth.me.success", me)
}
