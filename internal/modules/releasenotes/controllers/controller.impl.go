package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/repositories"
	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

type ReleaseNoteControllerImpl struct {
	ReleaseNoteService services.ReleaseNoteService
	response           *response.Sender
	log                *logger.LayerLogger
}

func NewReleaseNoteController(releaseNoteService services.ReleaseNoteService, sender *response.Sender, appLogger *logger.Logger) ReleaseNoteController {
	return &ReleaseNoteControllerImpl{
		ReleaseNoteService: releaseNoteService,
		response:           sender,
		log:                appLogger.Layer("controller.release_notes"),
	}
}

func (c *ReleaseNoteControllerImpl) Find(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Find")

	limit := parseUintQuery(r, "limit", 20)
	offset := parseUintQuery(r, "offset", 0)
	releaseNotes, err := c.ReleaseNoteService.Find(r.Context(), limit, offset)
	if err != nil {
		end(err)
		c.response.Error(w, r, http.StatusInternalServerError, "error.internal", nil)
		return
	}

	end(nil, "count", len(releaseNotes))
	c.response.Success(w, r, http.StatusOK, "release_notes.find.success", dto.ListResponse[entities.ReleaseNote]{Items: releaseNotes})
}

func (c *ReleaseNoteControllerImpl) FindByID(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "FindByID")

	id, err := parseID(r)
	if err != nil {
		end(err)
		c.response.Error(w, r, http.StatusBadRequest, "release_notes.invalid_id", nil)
		return
	}

	releaseNote, err := c.ReleaseNoteService.FindByID(r.Context(), id)
	if errors.Is(err, repositories.ErrNotFound) {
		end(err)
		c.response.Error(w, r, http.StatusNotFound, "release_notes.not_found", nil)
		return
	}
	if err != nil {
		end(err)
		c.response.Error(w, r, http.StatusInternalServerError, "error.internal", nil)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "release_notes.find_by_id.success", releaseNote)
}

func (c *ReleaseNoteControllerImpl) Create(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Create")

	var request dto.CreateReleaseNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		end(err)
		c.response.Error(w, r, http.StatusBadRequest, "release_notes.invalid_payload", nil)
		return
	}

	releaseNote, err := c.ReleaseNoteService.Create(r.Context(), request)
	if err != nil {
		end(err)
		c.response.Error(w, r, http.StatusBadRequest, "release_notes.create.failed", nil)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusCreated, "release_notes.create.success", releaseNote)
}

func (c *ReleaseNoteControllerImpl) Update(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Update")

	id, err := parseID(r)
	if err != nil {
		end(err)
		c.response.Error(w, r, http.StatusBadRequest, "release_notes.invalid_id", nil)
		return
	}

	var request dto.UpdateReleaseNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		end(err)
		c.response.Error(w, r, http.StatusBadRequest, "release_notes.invalid_payload", nil)
		return
	}

	releaseNote, err := c.ReleaseNoteService.Update(r.Context(), id, request)
	if errors.Is(err, repositories.ErrNotFound) {
		end(err)
		c.response.Error(w, r, http.StatusNotFound, "release_notes.not_found", nil)
		return
	}
	if err != nil {
		end(err)
		c.response.Error(w, r, http.StatusBadRequest, "release_notes.update.failed", nil)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "release_notes.update.success", releaseNote)
}

func (c *ReleaseNoteControllerImpl) Delete(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Delete")

	id, err := parseID(r)
	if err != nil {
		end(err)
		c.response.Error(w, r, http.StatusBadRequest, "release_notes.invalid_id", nil)
		return
	}

	if err := c.ReleaseNoteService.Delete(r.Context(), id, nil); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			end(err)
			c.response.Error(w, r, http.StatusNotFound, "release_notes.not_found", nil)
			return
		}
		end(err)
		c.response.Error(w, r, http.StatusInternalServerError, "error.internal", nil)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "release_notes.delete.success", nil)
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func parseUintQuery(r *http.Request, key string, fallback uint64) uint64 {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fallback
	}

	return parsed
}
