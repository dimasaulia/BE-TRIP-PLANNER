package controllers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/open-suite/boilerplate-golang/internal/modules/uploads/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

// multipartSlack covers boundaries and headers around the file itself.
const multipartSlack = 64 * 1024

type UploadControllerImpl struct {
	UploadService services.UploadService
	response      *response.Sender
	maxBytes      int64
	log           *logger.LayerLogger
}

func NewUploadController(uploadService services.UploadService, sender *response.Sender, cfg config.Config, appLogger *logger.Logger) UploadController {
	return &UploadControllerImpl{
		UploadService: uploadService,
		response:      sender,
		maxBytes:      cfg.Upload.MaxBytes,
		log:           appLogger.Layer("controller.uploads"),
	}
}

func (c *UploadControllerImpl) Upload(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Upload")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "id")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	// Cap the body before reading anything so an oversized file is refused
	// with 413 without being buffered in memory.
	r.Body = http.MaxBytesReader(w, r.Body, c.maxBytes+multipartSlack)

	reader, err := r.MultipartReader()
	if err != nil {
		end(err)
		c.response.Fail(w, r, apperror.BadRequest("invalid_upload").Wrap(err))
		return
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			c.response.Fail(w, r, apperror.BadRequest("invalid_upload").With("reason", "missing file field"))
			end(err)
			return
		}
		if err != nil {
			end(err)
			c.response.Fail(w, r, uploadReadError(err))
			return
		}
		if part.FormName() != "file" {
			continue
		}

		result, err := c.UploadService.Upload(r.Context(), user.ID, tripID, part.FileName(), part)
		if err != nil {
			end(err)
			c.response.Fail(w, r, uploadReadError(err))
			return
		}

		end(nil)
		c.response.Success(w, r, http.StatusCreated, "uploads.create.success", result)
		return
	}
}

func uploadReadError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return apperror.TooLarge("payload_too_large").With("max_bytes", tooLarge.Limit)
	}
	return err
}

func (c *UploadControllerImpl) Serve(w http.ResponseWriter, r *http.Request) {
	end := c.log.Start(r.Context(), "Serve")
	user, _ := requestctx.UserFrom(r.Context())

	tripID, err := httpx.PathUUID(r, "tripId")
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}

	file, attachment, err := c.UploadService.Open(r.Context(), user.ID, tripID, r.PathValue("storedName"))
	if err != nil {
		end(err)
		c.response.Fail(w, r, err)
		return
	}
	defer file.Close()

	// The stored name is a never reused UUID, so the response can be cached forever.
	w.Header().Set("Content-Type", attachment.Mime)
	w.Header().Set("Content-Length", strconv.FormatInt(attachment.SizeBytes, 10))
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	end(nil)
	http.ServeContent(w, r, attachment.StoredName, attachment.CreatedAt, file)
}
