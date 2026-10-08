package app_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/repositories"
	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/services"
	"github.com/open-suite/boilerplate-golang/internal/platform/crypto"
	"github.com/open-suite/boilerplate-golang/internal/platform/google"
)

// fakeGoogle stands in for the Calendar API and records what the worker does.
type fakeGoogle struct {
	mu     sync.Mutex
	events map[string]google.Event
	calls  []string
	fail   func(op string) error // returns an error to inject for an operation
}

func newFakeGoogle() *fakeGoogle {
	return &fakeGoogle{events: map[string]google.Event{}}
}

func (f *fakeGoogle) Calendar(context.Context, string) google.CalendarAPI { return f }

func (f *fakeGoogle) record(op string) error {
	f.calls = append(f.calls, op)
	if f.fail != nil {
		return f.fail(op)
	}
	return nil
}

func (f *fakeGoogle) CreateCalendar(context.Context, string, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return "cal-1", f.record("createCalendar")
}

func (f *fakeGoogle) DeleteCalendar(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.record("deleteCalendar")
}

func (f *fakeGoogle) InsertEvent(_ context.Context, _ string, event google.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.record("insert"); err != nil {
		return err
	}
	if _, exists := f.events[event.ID]; exists {
		return &google.APIError{Status: http.StatusConflict}
	}
	f.events[event.ID] = event
	return nil
}

func (f *fakeGoogle) UpdateEvent(_ context.Context, _ string, event google.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.record("update"); err != nil {
		return err
	}
	if _, exists := f.events[event.ID]; !exists {
		return &google.APIError{Status: http.StatusNotFound}
	}
	f.events[event.ID] = event
	return nil
}

func (f *fakeGoogle) DeleteEvent(_ context.Context, _ string, eventID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.record("delete"); err != nil {
		return err
	}
	if _, exists := f.events[eventID]; !exists {
		return &google.APIError{Status: http.StatusGone}
	}
	delete(f.events, eventID)
	return nil
}

func (f *fakeGoogle) count(op string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	total := 0
	for _, call := range f.calls {
		if call == op {
			total++
		}
	}
	return total
}

func (f *fakeGoogle) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
	f.fail = nil
}

type calendarHarness struct {
	t      *testing.T
	owner  *client
	tripID string
	syncID uuid.UUID
	fake   *fakeGoogle
	worker *services.Worker
}

// newCalendarHarness gives a user an active Google connection and a sync on a
// fresh trip, bypassing the browser-only OAuth consent.
func newCalendarHarness(t *testing.T) *calendarHarness {
	t.Helper()
	ctx := context.Background()

	owner := newClient(t)
	tripID := owner.newTrip()

	keys, err := crypto.New(testCfg)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := keys.Encrypt([]byte("refresh-token"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := testDB.Pool.Exec(ctx,
		`INSERT INTO calendar_connections (user_id, google_email, refresh_token_enc, scopes, status)
		 VALUES ($1, $2, $3, 'calendar.app.created', 'active')`, owner.userID, owner.email, encrypted); err != nil {
		t.Fatal(err)
	}

	var syncID uuid.UUID
	if err := testDB.Pool.QueryRow(ctx,
		`INSERT INTO calendar_syncs (trip_id, user_id, google_calendar_id) VALUES ($1, $2, 'cal-1') RETURNING id`,
		tripID, owner.userID).Scan(&syncID); err != nil {
		t.Fatal(err)
	}

	fake := newFakeGoogle()
	return &calendarHarness{
		t: t, owner: owner, tripID: tripID, syncID: syncID, fake: fake,
		worker: services.NewWorkerWithProvider(
			repositories.NewCalendarRepository(testDB, testLog), fake, keys, testCfg.App.BaseURL, testLog),
	}
}

func (h *calendarHarness) exec(query string, args ...any) {
	h.t.Helper()
	if _, err := testDB.Pool.Exec(context.Background(), query, args...); err != nil {
		h.t.Fatal(err)
	}
}

func (h *calendarHarness) scalar(query string, args ...any) int {
	h.t.Helper()
	var value int
	if err := testDB.Pool.QueryRow(context.Background(), query, args...).Scan(&value); err != nil {
		h.t.Fatal(err)
	}
	return value
}

// drain makes every job due (skipping the 5 second debounce) and runs the worker until idle.
func (h *calendarHarness) drain() {
	h.t.Helper()
	h.exec(`UPDATE sync_jobs SET run_after = NOW() - INTERVAL '1 second' WHERE sync_id = $1 AND locked_until IS NULL`, h.syncID)

	for range 20 {
		processed, err := h.worker.RunBatch(context.Background())
		if err != nil {
			h.t.Fatal(err)
		}
		if processed == 0 {
			return
		}
		h.exec(`UPDATE sync_jobs SET run_after = NOW() - INTERVAL '1 second' WHERE sync_id = $1 AND locked_until IS NULL AND attempts = 0`, h.syncID)
	}
}

func (h *calendarHarness) event(blockID string) (google.Event, bool) {
	h.fake.mu.Lock()
	defer h.fake.mu.Unlock()

	id, _ := uuid.Parse(blockID)
	event, ok := h.fake.events[services.EventID(h.syncID, id)]
	return event, ok
}

func TestCalendarSyncLifecycle(t *testing.T) {
	h := newCalendarHarness(t)

	a := h.owner.newBlock(h.tripID, "2026-12-14T03:00:00Z", "2026-12-14T04:00:00Z", "A").expect(t, http.StatusCreated).str("data", "id")
	b := h.owner.newBlock(h.tripID, "2026-12-14T05:00:00Z", "2026-12-14T06:00:00Z", "B").expect(t, http.StatusCreated).str("data", "id")
	c := h.owner.newBlock(h.tripID, "2026-12-14T07:00:00Z", "2026-12-14T08:00:00Z", "C").expect(t, http.StatusCreated).str("data", "id")

	t.Run("many edits collapse into one delivery per block", func(t *testing.T) {
		version := 1
		for i := range 20 {
			start := "2026-12-14T09:00:00Z"
			end := "2026-12-14T10:00:00Z"
			if i%2 == 1 {
				start, end = "2026-12-14T11:00:00Z", "2026-12-14T12:00:00Z"
			}
			h.owner.do(http.MethodPatch, "/schedule/blocks/"+a, map[string]any{"start_at": start, "end_at": end, "version": version}).
				expect(t, http.StatusOK)
			version++
		}

		if pending := h.scalar(`SELECT COUNT(*) FROM sync_jobs WHERE sync_id = $1 AND op = 'upsert'`, h.syncID); pending != 3 {
			t.Fatalf("3 blocks must mean 3 pending jobs, got %d", pending)
		}

		h.drain()

		if inserts := h.fake.count("insert"); inserts != 3 {
			t.Fatalf("expected exactly 3 inserts, got %d (calls: %v)", inserts, h.fake.calls)
		}
		event, ok := h.event(a)
		if !ok || event.Start.Format(time.RFC3339) != "2026-12-14T11:00:00Z" || event.TimeZone != "Asia/Jakarta" || event.Summary != "A" {
			t.Fatalf("the event must carry the final state of the block, got %+v", event)
		}
		if links := h.scalar(`SELECT COUNT(*) FROM calendar_event_links WHERE sync_id = $1`, h.syncID); links != 3 {
			t.Fatalf("expected 3 links, got %d", links)
		}
		if synced := h.scalar(`SELECT COUNT(*) FROM calendar_syncs WHERE id = $1 AND last_synced_at IS NOT NULL AND last_error IS NULL`, h.syncID); synced != 1 {
			t.Fatal("last_synced_at must be recorded")
		}
	})

	t.Run("a full resync of unchanged data sends nothing", func(t *testing.T) {
		h.fake.reset()
		h.owner.do(http.MethodPost, "/trips/"+h.tripID+"/calendar-sync/resync", nil).expect(t, http.StatusAccepted)
		h.drain()

		if h.fake.count("insert")+h.fake.count("update")+h.fake.count("delete") != 0 {
			t.Fatalf("content hashes must skip unchanged events, calls: %v", h.fake.calls)
		}
	})

	t.Run("move updates, delete removes", func(t *testing.T) {
		h.fake.reset()
		h.owner.do(http.MethodPatch, "/schedule/blocks/"+b, map[string]any{
			"start_at": "2026-12-14T13:00:00Z", "end_at": "2026-12-14T14:00:00Z", "version": 1,
		}).expect(t, http.StatusOK)
		h.owner.do(http.MethodDelete, "/schedule/blocks/"+c, nil).expect(t, http.StatusOK)
		h.drain()

		if h.fake.count("update") != 1 || h.fake.count("delete") != 1 {
			t.Fatalf("expected one update and one delete, got %v", h.fake.calls)
		}
		if moved, _ := h.event(b); moved.Start.Format(time.RFC3339) != "2026-12-14T13:00:00Z" {
			t.Fatalf("event was not moved: %+v", moved)
		}
		if _, exists := h.event(c); exists {
			t.Fatal("deleted block must disappear from Google")
		}
		if links := h.scalar(`SELECT COUNT(*) FROM calendar_event_links WHERE sync_id = $1`, h.syncID); links != 2 {
			t.Fatalf("link of the deleted block must go, got %d", links)
		}
	})

	t.Run("editing the library item refreshes its blocks", func(t *testing.T) {
		itemID := h.owner.newItem(h.tripID, "Old name")
		blockID := h.owner.do(http.MethodPost, "/trips/"+h.tripID+"/schedule/blocks", map[string]any{
			"library_item_id": itemID, "start_at": "2026-12-15T03:00:00Z", "end_at": "2026-12-15T04:00:00Z",
		}).expect(t, http.StatusCreated).str("data", "id")
		h.drain()
		if event, _ := h.event(blockID); event.Summary != "Old name" {
			t.Fatalf("summary follows the item title: %+v", event)
		}

		h.owner.do(http.MethodPatch, "/library/"+itemID, map[string]any{"version": 1, "title": "New name"}).expect(t, http.StatusOK)
		h.drain()
		if event, _ := h.event(blockID); event.Summary != "New name" {
			t.Fatalf("renaming the item must update Google: %+v", event)
		}
	})

	t.Run("an event deleted in Google is created again", func(t *testing.T) {
		h.fake.mu.Lock()
		id, _ := uuid.Parse(b)
		delete(h.fake.events, services.EventID(h.syncID, id))
		h.fake.mu.Unlock()
		h.fake.reset()

		h.owner.do(http.MethodPatch, "/schedule/blocks/"+b, map[string]any{"note": "touch", "version": 2}).expect(t, http.StatusOK)
		h.drain()

		if _, ok := h.event(b); !ok || h.fake.count("insert") != 1 {
			t.Fatalf("the missing event must be recreated, calls: %v", h.fake.calls)
		}
	})

	t.Run("an existing id is treated as already created", func(t *testing.T) {
		h.fake.reset()
		blockID := h.owner.newBlock(h.tripID, "2026-12-16T03:00:00Z", "2026-12-16T04:00:00Z", "D").expect(t, http.StatusCreated).str("data", "id")

		// Simulate a delivery whose response was lost: Google has the event, we have no link.
		id, _ := uuid.Parse(blockID)
		h.fake.mu.Lock()
		h.fake.events[services.EventID(h.syncID, id)] = google.Event{ID: services.EventID(h.syncID, id), Summary: "stale"}
		h.fake.mu.Unlock()

		h.drain()
		if h.fake.count("insert") != 1 || h.fake.count("update") != 1 {
			t.Fatalf("409 on insert must fall through to update, got %v", h.fake.calls)
		}
		if event, _ := h.event(blockID); event.Summary != "D" {
			t.Fatalf("the stale event must be overwritten: %+v", event)
		}
	})
}

func TestCalendarSyncFailures(t *testing.T) {
	t.Run("transient errors back off and succeed later", func(t *testing.T) {
		h := newCalendarHarness(t)
		h.owner.newBlock(h.tripID, "2026-12-14T03:00:00Z", "2026-12-14T04:00:00Z", "A").expect(t, http.StatusCreated)

		h.fake.fail = func(op string) error { return &google.APIError{Status: http.StatusServiceUnavailable} }
		h.drain()

		if attempts := h.scalar(`SELECT attempts FROM sync_jobs WHERE sync_id = $1`, h.syncID); attempts != 1 {
			t.Fatalf("the job must stay queued with attempts=1, got %d", attempts)
		}
		if future := h.scalar(`SELECT COUNT(*) FROM sync_jobs WHERE sync_id = $1 AND run_after > NOW() + INTERVAL '20 seconds'`, h.syncID); future != 1 {
			t.Fatal("the retry must be pushed back by the backoff")
		}
		if failing := h.scalar(`SELECT COUNT(*) FROM calendar_syncs WHERE id = $1 AND last_error IS NOT NULL`, h.syncID); failing != 1 {
			t.Fatal("the failure must be visible in last_error")
		}

		h.fake.reset()
		h.exec(`UPDATE sync_jobs SET run_after = NOW() WHERE sync_id = $1`, h.syncID)
		h.drain()
		if left := h.scalar(`SELECT COUNT(*) FROM sync_jobs WHERE sync_id = $1`, h.syncID); left != 0 {
			t.Fatalf("the job must finish once Google recovers, %d left", left)
		}
		if cleared := h.scalar(`SELECT COUNT(*) FROM calendar_syncs WHERE id = $1 AND last_error IS NULL`, h.syncID); cleared != 1 {
			t.Fatal("last_error must clear after a success")
		}
	})

	t.Run("gives up after 8 attempts", func(t *testing.T) {
		h := newCalendarHarness(t)
		h.owner.newBlock(h.tripID, "2026-12-14T03:00:00Z", "2026-12-14T04:00:00Z", "A").expect(t, http.StatusCreated)
		h.fake.fail = func(op string) error { return &google.APIError{Status: http.StatusTooManyRequests} }

		for range 9 {
			h.exec(`UPDATE sync_jobs SET run_after = NOW() - INTERVAL '1 second', locked_until = NULL WHERE sync_id = $1`, h.syncID)
			if _, err := h.worker.RunBatch(context.Background()); err != nil {
				t.Fatal(err)
			}
		}

		if left := h.scalar(`SELECT COUNT(*) FROM sync_jobs WHERE sync_id = $1`, h.syncID); left != 0 {
			t.Fatalf("a job that keeps failing must be dropped, %d left", left)
		}
	})

	t.Run("a permanent error is not retried", func(t *testing.T) {
		h := newCalendarHarness(t)
		h.owner.newBlock(h.tripID, "2026-12-14T03:00:00Z", "2026-12-14T04:00:00Z", "A").expect(t, http.StatusCreated)
		h.fake.fail = func(op string) error { return &google.APIError{Status: http.StatusForbidden, Body: "nope"} }

		h.drain()
		if left := h.scalar(`SELECT COUNT(*) FROM sync_jobs WHERE sync_id = $1`, h.syncID); left != 0 {
			t.Fatal("a 403 must not be retried")
		}
		if failing := h.scalar(`SELECT COUNT(*) FROM calendar_syncs WHERE id = $1 AND last_error IS NOT NULL`, h.syncID); failing != 1 {
			t.Fatal("the reason must be recorded")
		}
	})

	t.Run("a revoked grant disables the connection and surfaces in /me", func(t *testing.T) {
		h := newCalendarHarness(t)
		h.owner.newBlock(h.tripID, "2026-12-14T03:00:00Z", "2026-12-14T04:00:00Z", "A").expect(t, http.StatusCreated)
		h.owner.newBlock(h.tripID, "2026-12-14T05:00:00Z", "2026-12-14T06:00:00Z", "B").expect(t, http.StatusCreated)
		h.fake.fail = func(op string) error { return google.ErrInvalidGrant }

		h.drain()

		me := h.owner.do(http.MethodGet, "/me", nil).expect(t, http.StatusOK)
		if me.str("data", "calendar", "status") != "revoked" || get(me.Body, "data", "calendar", "connected") != false {
			t.Fatalf("/me must ask the user to reconnect: %s", me.Raw)
		}
		if left := h.scalar(`SELECT COUNT(*) FROM sync_jobs WHERE sync_id = $1`, h.syncID); left != 0 {
			t.Fatal("pending jobs of a revoked connection must stop")
		}

		detail := h.owner.do(http.MethodGet, "/trips/"+h.tripID, nil).expect(t, http.StatusOK)
		if detail.str("data", "calendar_sync", "last_error") == "" {
			t.Fatalf("the trip must show why syncing stopped: %s", detail.Raw)
		}
	})
}

func TestCalendarEndpoints(t *testing.T) {
	owner, viewer := newClient(t), newClient(t)
	tripID := owner.newTrip()
	owner.invite(tripID, "viewer", viewer)

	t.Run("enabling needs a connected Google account", func(t *testing.T) {
		owner.do(http.MethodPut, "/trips/"+tripID+"/calendar-sync", nil).
			expectError(t, http.StatusUnprocessableEntity, "calendar_not_connected")
	})

	t.Run("disable and resync need an active sync", func(t *testing.T) {
		owner.do(http.MethodDelete, "/trips/"+tripID+"/calendar-sync", nil).expectError(t, http.StatusNotFound, "calendar_sync_not_found")
		owner.do(http.MethodPost, "/trips/"+tripID+"/calendar-sync/resync", nil).expectError(t, http.StatusNotFound, "calendar_sync_not_found")
	})

	t.Run("every role may sync for themselves, strangers may not", func(t *testing.T) {
		outsider := newClient(t)
		outsider.do(http.MethodPut, "/trips/"+tripID+"/calendar-sync", nil).expectError(t, http.StatusNotFound, "trip_not_found")

		// A viewer reaches the connection check, i.e. role is not what stops them.
		viewer.do(http.MethodPut, "/trips/"+tripID+"/calendar-sync", nil).expectError(t, http.StatusUnprocessableEntity, "calendar_not_connected")
	})

	t.Run("connect needs Google credentials", func(t *testing.T) {
		owner.do(http.MethodGet, "/calendar/connect", nil).expectError(t, http.StatusServiceUnavailable, "google_not_configured")
	})

	t.Run("disconnect is safe when nothing is connected", func(t *testing.T) {
		owner.do(http.MethodDelete, "/calendar/connection", nil).expect(t, http.StatusOK)
	})

	t.Run("a member who leaves loses their sync", func(t *testing.T) {
		h := newCalendarHarness(t)
		other := newClient(t)
		h.owner.invite(h.tripID, "editor", other)
		h.owner.do(http.MethodDelete, "/trips/"+h.tripID+"/members/"+h.owner.userID, nil).
			expectError(t, http.StatusUnprocessableEntity, "last_owner")

		h.owner.do(http.MethodPatch, "/trips/"+h.tripID+"/members/"+other.userID, map[string]any{"role": "owner"}).expect(t, http.StatusOK)
		h.owner.do(http.MethodDelete, "/trips/"+h.tripID+"/members/"+h.owner.userID, nil).expect(t, http.StatusOK)

		if left := h.scalar(`SELECT COUNT(*) FROM calendar_syncs WHERE id = $1`, h.syncID); left != 0 {
			t.Fatal("a removed member must not keep syncing the trip")
		}
	})
}
