package controllers

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/library/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/library/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

type LibraryControllerImpl struct {
	LibraryService services.LibraryService
	response       *response.Sender
	log            *logger.LayerLogger
}

func NewLibraryController(libraryService services.LibraryService, sender *response.Sender, appLogger *logger.Logger) LibraryController {
	return &LibraryControllerImpl{
		LibraryService: libraryService,
		response:       sender,
		log:            appLogger.Layer("controller.library"),
	}
}

func (c *LibraryControllerImpl) List(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "List")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	query := r.URL.Query()
	result, err := c.LibraryService.List(r.Context(), user.ID, tripID,
		query.Get("kind"), query.Get("q"), query.Get("cursor"), httpx.QueryInt(r, "limit", 50, 1, 100))
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "library.list.success", result)
}

func (c *LibraryControllerImpl) Create(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Create")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	var request dto.CreateItemRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	item, err := c.LibraryService.Create(r.Context(), user.ID, tripID, request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusCreated, "library.create.success", item)
}

func (c *LibraryControllerImpl) Get(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Get")
	user, _ := requestctx.UserFrom(r.Context())

	itemID, err := httpx.PathUUID(r, "itemId")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	item, err := c.LibraryService.Get(r.Context(), user.ID, itemID)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "library.get.success", item)
}

func (c *LibraryControllerImpl) Update(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Update")
	user, _ := requestctx.UserFrom(r.Context())

	itemID, err := httpx.PathUUID(r, "itemId")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	var request dto.UpdateItemRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	item, err := c.LibraryService.Update(r.Context(), user.ID, itemID, request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "library.update.success", item)
}

func (c *LibraryControllerImpl) Delete(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Delete")
	user, _ := requestctx.UserFrom(r.Context())

	itemID, err := httpx.PathUUID(r, "itemId")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	if err := c.LibraryService.Delete(r.Context(), user.ID, itemID, httpx.QueryBool(r, "force")); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusOK, "library.delete.success", nil)
}

func (c *LibraryControllerImpl) Export(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Export")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	var request dto.ExportRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	result, err := c.LibraryService.Export(r.Context(), user.ID, tripID, request)
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	end(nil)
	c.response.Success(w, r, http.StatusCreated, "library.export.success", result)
}
