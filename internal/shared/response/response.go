package response

import (
	"encoding/json"
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/platform/i18n"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/sanitizer"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
)

type Sender struct {
	translator *i18n.Translator
	log        *logger.LayerLogger
}

type Body struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func NewSender(translator *i18n.Translator, appLogger *logger.Logger) *Sender {
	return &Sender{
		translator: translator,
		log:        appLogger.Layer("shared.response"),
	}
}

func (s *Sender) Success(w http.ResponseWriter, r *http.Request, statusCode int, messageKey string, data any) {
	message := s.translator.Translate(requestctx.Language(r.Context()), messageKey, nil)
	body := Body{
		Success: true,
		Message: message,
		Data:    data,
	}

	s.write(w, r, statusCode, messageKey, body)
}

func (s *Sender) Error(w http.ResponseWriter, r *http.Request, statusCode int, messageKey string, data any) {
	message := s.translator.Translate(requestctx.Language(r.Context()), messageKey, nil)
	body := Body{
		Success: false,
		Message: message,
		Data:    data,
	}

	s.write(w, r, statusCode, messageKey, body)
}

func (s *Sender) write(w http.ResponseWriter, r *http.Request, statusCode int, messageKey string, body Body) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		s.log.Error(r.Context(), "write", err)
		return
	}

	s.log.Info(
		r.Context(),
		"write",
		"status", statusCode,
		"success", body.Success,
		"message_key", messageKey,
		"data", sanitizer.Value("data", body.Data),
	)
}
