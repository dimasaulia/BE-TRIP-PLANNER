package google

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type recorded struct {
	Method string
	Path   string
	Body   map[string]any
}

func fakeServer(t *testing.T, status int, response string) (*httptest.Server, *[]recorded) {
	t.Helper()

	var requests []recorded
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)

		requests = append(requests, recorded{Method: r.Method, Path: r.URL.Path, Body: body})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func TestCalendarAPIRequests(t *testing.T) {
	server, requests := fakeServer(t, http.StatusOK, `{"id":"abc@group.calendar.google.com"}`)
	api := NewCalendarAPI(server.Client(), server.URL)
	ctx := context.Background()

	id, err := api.CreateCalendar(ctx, "Bali", "Asia/Jakarta")
	if err != nil || id != "abc@group.calendar.google.com" {
		t.Fatalf("create calendar: %q %v", id, err)
	}

	event := Event{
		ID: "abc123", Summary: "Pantai", Location: "Kuta", Description: "desc",
		Start:    time.Date(2026, 12, 14, 3, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 12, 14, 5, 0, 0, 0, time.UTC),
		TimeZone: "Asia/Jakarta",
	}
	if err := api.InsertEvent(ctx, id, event); err != nil {
		t.Fatal(err)
	}
	if err := api.UpdateEvent(ctx, id, event); err != nil {
		t.Fatal(err)
	}
	if err := api.DeleteEvent(ctx, id, "abc123"); err != nil {
		t.Fatal(err)
	}
	if err := api.DeleteCalendar(ctx, id); err != nil {
		t.Fatal(err)
	}

	calendarPath := "/calendars/abc@group.calendar.google.com"
	want := []struct{ method, path string }{
		{"POST", "/calendars"},
		{"POST", calendarPath + "/events"},
		{"PUT", calendarPath + "/events/abc123"},
		{"DELETE", calendarPath + "/events/abc123"},
		{"DELETE", calendarPath},
	}
	if len(*requests) != len(want) {
		t.Fatalf("expected %d requests, got %d", len(want), len(*requests))
	}
	for i, expected := range want {
		got := (*requests)[i]
		if got.Method != expected.method || got.Path != expected.path {
			t.Errorf("request %d = %s %s, want %s %s", i, got.Method, got.Path, expected.method, expected.path)
		}
	}

	created := (*requests)[0].Body
	if created["summary"] != "Bali" || created["timeZone"] != "Asia/Jakarta" {
		t.Errorf("calendar body: %v", created)
	}

	sent := (*requests)[1].Body
	start, _ := sent["start"].(map[string]any)
	if sent["id"] != "abc123" || sent["status"] != "confirmed" || sent["summary"] != "Pantai" ||
		start["dateTime"] != "2026-12-14T03:00:00Z" || start["timeZone"] != "Asia/Jakarta" {
		t.Errorf("event body: %v", sent)
	}
}

func TestCalendarAPIErrors(t *testing.T) {
	tests := []struct {
		status    int
		retryable bool
		gone      bool
	}{
		{http.StatusTooManyRequests, true, false},
		{http.StatusInternalServerError, true, false},
		{http.StatusBadGateway, true, false},
		{http.StatusForbidden, false, false},
		{http.StatusConflict, false, false},
		{http.StatusNotFound, false, true},
		{http.StatusGone, false, true},
	}

	for _, test := range tests {
		server, _ := fakeServer(t, test.status, `{"error":"x"}`)
		err := NewCalendarAPI(server.Client(), server.URL).DeleteEvent(context.Background(), "c", "e")

		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != test.status {
			t.Fatalf("status %d: expected APIError, got %v", test.status, err)
		}
		if apiErr.Retryable() != test.retryable || apiErr.Gone() != test.gone {
			t.Errorf("status %d: retryable=%v gone=%v", test.status, apiErr.Retryable(), apiErr.Gone())
		}
	}
}

// A revoked or expired refresh token surfaces as ErrInvalidGrant, which the
// worker turns into "reconnect Google Calendar".
func TestInvalidGrantIsRecognised(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`))
	}))
	defer tokenServer.Close()

	config := &oauth2.Config{
		ClientID: "id", ClientSecret: "secret",
		Endpoint: oauth2.Endpoint{TokenURL: tokenServer.URL, AuthStyle: oauth2.AuthStyleInParams},
	}
	source := config.TokenSource(context.Background(), &oauth2.Token{RefreshToken: "revoked"})

	api := NewCalendarAPI(oauth2.NewClient(context.Background(), source), "http://127.0.0.1:1")
	if _, err := api.CreateCalendar(context.Background(), "x", "UTC"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("expected ErrInvalidGrant, got %v", err)
	}
}
