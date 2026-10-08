package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func (c *client) dialWS(tripID string, origin string) (*websocket.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}

	return websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/trips/"+tripID+"/ws", &websocket.DialOptions{
		HTTPClient: c.http,
		HTTPHeader: header,
	})
}

// wsClient reads in the background: coder/websocket closes the connection
// when a Read context expires, so tests must never read with a short timeout.
type wsClient struct {
	conn   *websocket.Conn
	msgs   chan message
	closed chan struct{}
}

func (c *client) connect(tripID string) *wsClient {
	c.t.Helper()

	conn, _, err := c.dialWS(tripID, testOrigin)
	if err != nil {
		c.t.Fatalf("dial websocket: %v", err)
	}

	ws := &wsClient{conn: conn, msgs: make(chan message, 256), closed: make(chan struct{})}
	go func() {
		defer close(ws.closed)
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var parsed message
			_ = json.Unmarshal(data, &parsed)
			ws.msgs <- parsed
		}
	}()

	c.t.Cleanup(func() { conn.CloseNow() })
	return ws
}

type message map[string]any

func (m message) typ() string { s, _ := m["type"].(string); return s }
func (m message) seq() int64  { f, _ := m["seq"].(float64); return int64(f) }

// next returns the next message, or nil when nothing arrives in time.
func (w *wsClient) next(wait time.Duration) message {
	select {
	case m := <-w.msgs:
		return m
	case <-time.After(wait):
		return nil
	}
}

// waitFor skips other traffic until a message of the type shows up.
func waitFor(t *testing.T, ws *wsClient, typ string) message {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if m := ws.next(time.Until(deadline)); m != nil && m.typ() == typ {
			return m
		}
	}
	t.Fatalf("no %q message within 3s", typ)
	return nil
}

// waitForPresence skips presence updates until exactly n users are online.
func waitForPresence(t *testing.T, ws *wsClient, n int) message {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m := ws.next(time.Until(deadline))
		if m == nil || m.typ() != "presence.update" {
			continue
		}
		if users, _ := get(m, "payload", "users").([]any); len(users) == n {
			return m
		}
	}
	t.Fatalf("presence never reached %d users", n)
	return nil
}

func send(t *testing.T, ws *wsClient, typ string, payload any) {
	t.Helper()

	data, _ := json.Marshal(map[string]any{"type": typ, "payload": payload})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ws.conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
}

// expectSilence asserts the connection receives nothing of the given type.
func expectSilence(t *testing.T, ws *wsClient, typ string, wait time.Duration) {
	t.Helper()

	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if m := ws.next(time.Until(deadline)); m != nil && m.typ() == typ {
			t.Fatalf("unexpected %q message: %v", typ, m)
		}
	}
}

func TestRealtimeBroadcastAndPresence(t *testing.T) {
	owner, viewer, outsider := newClient(t), newClient(t), newClient(t)
	tripID := owner.newTrip()
	owner.invite(tripID, "viewer", viewer)

	ownerWS := owner.connect(tripID)
	hello := waitFor(t, ownerWS, "hello")
	if hello["v"] != float64(1) || get(hello, "payload", "role") != "owner" {
		t.Fatalf("unexpected hello: %v", hello)
	}

	viewerWS := viewer.connect(tripID)
	waitFor(t, viewerWS, "hello")

	t.Run("presence shows who is online and what they view", func(t *testing.T) {
		waitForPresence(t, ownerWS, 2)

		itemID := owner.newItem(tripID, "Watched")
		send(t, viewerWS, "presence.set", map[string]any{"viewing": map[string]any{"type": "library_item", "id": itemID}})

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			m := ownerWS.next(time.Until(deadline))
			if m == nil || m.typ() != "presence.update" {
				continue
			}
			for _, user := range get(m, "payload", "users").([]any) {
				if get(user, "user_id") == viewer.userID && get(user, "viewing", "id") == itemID {
					return
				}
			}
		}
		t.Fatal("owner never saw what the viewer is looking at")
	})

	t.Run("committed changes reach everyone after commit, tagged with the sender", func(t *testing.T) {
		before := waitFor(t, viewerWS, "library.created") // from newItem above
		created := owner.newBlock(tripID, "2026-12-14T03:00:00Z", "2026-12-14T05:00:00Z", "Pantai").expect(t, http.StatusCreated)

		got := waitFor(t, viewerWS, "block.created")
		if got["origin"] != owner.clientID || get(got, "payload", "id") != created.str("data", "id") {
			t.Fatalf("viewer must see the new block from the owner's client: %v", got)
		}
		if got.seq() <= before.seq() {
			t.Fatalf("seq must grow: %d then %d", before.seq(), got.seq())
		}
		// The sender gets the event too and can match it by origin.
		if echoed := waitFor(t, ownerWS, "block.created"); echoed["origin"] != owner.clientID {
			t.Fatalf("sender must receive its own event: %v", echoed)
		}

		blockID := created.str("data", "id")
		owner.do(http.MethodPatch, "/schedule/blocks/"+blockID, map[string]any{
			"start_at": "2026-12-14T04:00:00Z", "end_at": "2026-12-14T06:00:00Z", "version": 1,
		}).expect(t, http.StatusOK)
		updated := waitFor(t, viewerWS, "block.updated")
		if get(updated, "payload", "version") != float64(2) {
			t.Fatalf("event carries the full block and version: %v", updated)
		}

		owner.do(http.MethodDelete, "/schedule/blocks/"+blockID, nil).expect(t, http.StatusOK)
		waitFor(t, viewerWS, "block.deleted")
	})

	t.Run("a rejected change is never announced", func(t *testing.T) {
		owner.newBlock(tripID, "2026-12-15T03:00:00Z", "2026-12-15T05:00:00Z", "A").expect(t, http.StatusCreated)
		waitFor(t, viewerWS, "block.created")

		owner.newBlock(tripID, "2026-12-15T04:00:00Z", "2026-12-15T06:00:00Z", "clash").expect(t, http.StatusUnprocessableEntity)
		expectSilence(t, viewerWS, "block.created", 400*time.Millisecond)
	})

	t.Run("library events and the description rule", func(t *testing.T) {
		itemID := owner.newItem(tripID, "Library")
		waitFor(t, viewerWS, "library.created")

		owner.do(http.MethodPatch, "/library/"+itemID, map[string]any{"version": 1, "description_md": "only text"}).expect(t, http.StatusOK)
		expectSilence(t, viewerWS, "library.updated", 400*time.Millisecond)

		owner.do(http.MethodPatch, "/library/"+itemID, map[string]any{"version": 2, "title": "Retitled"}).expect(t, http.StatusOK)
		if m := waitFor(t, viewerWS, "library.updated"); get(m, "payload", "title") != "Retitled" {
			t.Fatalf("title change must broadcast: %v", m)
		}

		owner.do(http.MethodDelete, "/library/"+itemID, nil).expect(t, http.StatusOK)
		waitFor(t, viewerWS, "library.deleted")
	})

	t.Run("drag previews are relayed but not stored", func(t *testing.T) {
		blockID := "7c1e0000-0000-4000-8000-000000000001"
		send(t, ownerWS, "block.preview", map[string]any{"id": blockID, "start_at": "2026-12-14T06:00:00Z", "end_at": "2026-12-14T08:00:00Z"})

		preview := waitFor(t, viewerWS, "block.preview")
		if get(preview, "payload", "user_id") != owner.userID {
			t.Fatalf("preview names who is dragging: %v", preview)
		}
		expectSilence(t, ownerWS, "block.preview", 300*time.Millisecond) // the sender is not echoed

		list := owner.do(http.MethodGet, "/trips/"+tripID+"/schedule?from=2026-12-14T06:00:00Z&to=2026-12-14T08:00:00Z", nil).expect(t, http.StatusOK)
		if len(list.items()) != 0 {
			t.Fatalf("previews must not be persisted: %s", list.Raw)
		}

		send(t, ownerWS, "block.preview_end", map[string]any{"id": blockID})
		waitFor(t, viewerWS, "block.preview_end")
	})

	t.Run("viewers receive but cannot send previews", func(t *testing.T) {
		send(t, viewerWS, "block.preview", map[string]any{"id": "7c1e0000-0000-4000-8000-000000000002", "start_at": "2026-12-14T06:00:00Z", "end_at": "2026-12-14T08:00:00Z"})

		if rejected := waitFor(t, viewerWS, "error"); get(rejected, "payload", "code") != "forbidden" {
			t.Fatalf("unexpected error: %v", rejected)
		}
		expectSilence(t, ownerWS, "block.preview", 300*time.Millisecond)
	})

	t.Run("malformed and unknown messages get an error", func(t *testing.T) {
		send(t, ownerWS, "does.not.exist", nil)
		waitFor(t, ownerWS, "error")
	})

	t.Run("handshake checks origin and membership", func(t *testing.T) {
		if _, response, err := owner.dialWS(tripID, "https://evil.example"); err == nil || response == nil || response.StatusCode != http.StatusForbidden {
			t.Fatalf("a foreign origin must be refused, got %v %v", response, err)
		}
		if _, response, err := outsider.dialWS(tripID, testOrigin); err == nil || response == nil || response.StatusCode != http.StatusNotFound {
			t.Fatalf("a non member must be refused, got %v %v", response, err)
		}
	})

	t.Run("removed members are disconnected", func(t *testing.T) {
		owner.do(http.MethodDelete, "/trips/"+tripID+"/members/"+viewer.userID, nil).expect(t, http.StatusOK)

		changed := waitFor(t, ownerWS, "member.changed")
		if get(changed, "payload", "action") != "removed" {
			t.Fatalf("unexpected member event: %v", changed)
		}

		select {
		case <-viewerWS.closed:
		case <-time.After(3 * time.Second):
			t.Fatal("the removed member's socket must be closed")
		}
	})
}

// A refresh drops and re-opens the socket; the avatar must not blink, but a
// real departure disappears after the grace period.
func TestRealtimePresenceGracePeriod(t *testing.T) {
	owner, guest := newClient(t), newClient(t)
	tripID := owner.newTrip()
	owner.invite(tripID, "editor", guest)

	ownerWS := owner.connect(tripID)
	waitFor(t, ownerWS, "hello")

	guestWS := guest.connect(tripID)
	waitFor(t, guestWS, "hello")
	waitForPresence(t, ownerWS, 2)

	guestWS.conn.CloseNow()
	// Within the grace period nobody is told the guest left.
	expectSilence(t, ownerWS, "presence.update", 3*time.Second)

	// After it, the entry is dropped and everyone is told.
	waitForPresence(t, ownerWS, 1)
}
