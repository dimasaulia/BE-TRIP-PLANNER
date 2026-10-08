package google

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// ErrInvalidGrant means the refresh token was revoked or expired.
var ErrInvalidGrant = errors.New("google refresh token is no longer valid")

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("google calendar api returned %d: %s", e.Status, e.Body)
}

// Retryable errors are quota and server side problems worth another attempt.
func (e *APIError) Retryable() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= 500
}

func (e *APIError) Gone() bool {
	return e.Status == http.StatusNotFound || e.Status == http.StatusGone
}

type Event struct {
	ID          string
	Summary     string
	Location    string
	Description string
	Start       time.Time
	End         time.Time
	TimeZone    string
}

// CalendarAPI is the slice of the Calendar REST API the sync worker needs.
type CalendarAPI interface {
	CreateCalendar(ctx context.Context, summary string, timeZone string) (string, error)
	DeleteCalendar(ctx context.Context, calendarID string) error
	InsertEvent(ctx context.Context, calendarID string, event Event) error
	UpdateEvent(ctx context.Context, calendarID string, event Event) error
	DeleteEvent(ctx context.Context, calendarID string, eventID string) error
}

// CalendarProvider yields an API bound to one user's refresh token.
type CalendarProvider interface {
	Calendar(ctx context.Context, refreshToken string) CalendarAPI
}

// Calendar builds an API whose HTTP client refreshes access tokens on demand.
func (c *Client) Calendar(ctx context.Context, refreshToken string) CalendarAPI {
	source := c.CalendarConfig().TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	return NewCalendarAPI(oauth2.NewClient(ctx, source), c.cfg.CalendarAPIBase)
}

// RevokeToken tells Google to invalidate a refresh token.
func (c *Client) RevokeToken(ctx context.Context, token string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/revoke?token="+url.QueryEscape(token), nil)
	if err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		return &APIError{Status: response.StatusCode}
	}
	return nil
}

type httpCalendarAPI struct {
	client *http.Client
	base   string
}

func NewCalendarAPI(client *http.Client, base string) CalendarAPI {
	return &httpCalendarAPI{client: client, base: base}
}

func (a *httpCalendarAPI) CreateCalendar(ctx context.Context, summary string, timeZone string) (string, error) {
	var created struct {
		ID string `json:"id"`
	}
	err := a.do(ctx, http.MethodPost, "/calendars", map[string]string{
		"summary":  summary,
		"timeZone": timeZone,
	}, &created)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (a *httpCalendarAPI) DeleteCalendar(ctx context.Context, calendarID string) error {
	return a.do(ctx, http.MethodDelete, "/calendars/"+url.PathEscape(calendarID), nil, nil)
}

func (a *httpCalendarAPI) InsertEvent(ctx context.Context, calendarID string, event Event) error {
	return a.do(ctx, http.MethodPost, "/calendars/"+url.PathEscape(calendarID)+"/events", eventBody(event), nil)
}

func (a *httpCalendarAPI) UpdateEvent(ctx context.Context, calendarID string, event Event) error {
	return a.do(ctx, http.MethodPut, "/calendars/"+url.PathEscape(calendarID)+"/events/"+url.PathEscape(event.ID), eventBody(event), nil)
}

func (a *httpCalendarAPI) DeleteEvent(ctx context.Context, calendarID string, eventID string) error {
	return a.do(ctx, http.MethodDelete, "/calendars/"+url.PathEscape(calendarID)+"/events/"+url.PathEscape(eventID), nil, nil)
}

// eventBody always sends status=confirmed so updating an event that was
// cancelled earlier (same deterministic id) brings it back.
func eventBody(event Event) map[string]any {
	return map[string]any{
		"id":          event.ID,
		"status":      "confirmed",
		"summary":     event.Summary,
		"location":    event.Location,
		"description": event.Description,
		"start":       map[string]string{"dateTime": event.Start.UTC().Format(time.RFC3339), "timeZone": event.TimeZone},
		"end":         map[string]string{"dateTime": event.End.UTC().Format(time.RFC3339), "timeZone": event.TimeZone},
	}
}

func (a *httpCalendarAPI) do(ctx context.Context, method string, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, a.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := a.client.Do(request)
	if err != nil {
		var retrieveErr *oauth2.RetrieveError
		if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant" {
			return ErrInvalidGrant
		}
		return err
	}
	defer response.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 300 {
		return &APIError{Status: response.StatusCode, Body: string(payload)}
	}

	if out != nil {
		return json.Unmarshal(payload, out)
	}
	return nil
}
