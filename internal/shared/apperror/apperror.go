// Package apperror carries an HTTP status, a stable machine readable code and
// optional details from the service layer up to the response sender.
package apperror

import (
	"errors"
	"net/http"
)

type Error struct {
	Status  int
	Code    string
	Details map[string]any
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Err }

// With returns a copy carrying an extra detail entry.
func (e *Error) With(key string, value any) *Error {
	details := make(map[string]any, len(e.Details)+1)
	for k, v := range e.Details {
		details[k] = v
	}
	details[key] = value

	copied := *e
	copied.Details = details
	return &copied
}

// Wrap returns a copy that keeps the underlying cause for logging.
func (e *Error) Wrap(err error) *Error {
	copied := *e
	copied.Err = err
	return &copied
}

func As(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

func New(status int, code string) *Error {
	return &Error{Status: status, Code: code}
}

func BadRequest(code string) *Error            { return New(http.StatusBadRequest, code) }
func Unauthorized() *Error                     { return New(http.StatusUnauthorized, "unauthorized") }
func Forbidden(code string) *Error             { return New(http.StatusForbidden, code) }
func NotFound(code string) *Error              { return New(http.StatusNotFound, code) }
func Conflict(code string) *Error              { return New(http.StatusConflict, code) }
func TooLarge(code string) *Error              { return New(http.StatusRequestEntityTooLarge, code) }
func Unprocessable(code string) *Error         { return New(http.StatusUnprocessableEntity, code) }
func TooManyRequests(code string) *Error       { return New(http.StatusTooManyRequests, code) }
func Unavailable(code string) *Error           { return New(http.StatusServiceUnavailable, code) }
func UnsupportedMedia(code string) *Error      { return New(http.StatusUnsupportedMediaType, code) }
func Internal(err error) *Error                { return New(http.StatusInternalServerError, "internal").Wrap(err) }
func BadGateway(code string, err error) *Error { return New(http.StatusBadGateway, code).Wrap(err) }
