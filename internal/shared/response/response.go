package response

import (
	"encoding/json"
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/platform/i18n"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/sanitizer"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
)

type Sender struct {
	translator *i18n.Translator
	log        *logger.LayerLogger
}

// Body is the response envelope. Failures additionally carry Error, the
// machine readable {code, message, details} object frontends branch on.
type Body struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Data    any        `json:"data"`
	Error   *ErrorBody `json:"error,omitempty"`
}

type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
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

// Fail converts any error into the uniform error response. Unknown errors are
// logged and reported as a generic 500 so internals never leak.
func (s *Sender) Fail(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := apperror.As(err)
	if !ok {
		appErr = apperror.Internal(err)
	}
	if appErr.Status >= http.StatusInternalServerError {
		s.log.Error(r.Context(), "fail", appErr, "code", appErr.Code)
	}

	messageKey := "error." + appErr.Code
	message := s.translator.Translate(requestctx.Language(r.Context()), messageKey, nil)

	details := appErr.Details
	if details == nil {
		details = map[string]any{}
	}

	body := Body{
		Success: false,
		Message: message,
		Data:    nil,
		Error: &ErrorBody{
			Code:    appErr.Code,
			Message: message,
			Details: details,
		},
	}

	s.write(w, r, appErr.Status, messageKey, body)
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

// Redirect is used by the OAuth flows.
func (s *Sender) Redirect(w http.ResponseWriter, r *http.Request, target string) {
	http.Redirect(w, r, target, http.StatusFound)
}
