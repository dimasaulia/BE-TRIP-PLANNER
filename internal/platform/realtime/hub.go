// Package realtime is the in-memory WebSocket hub: one room per trip carrying
// presence, committed change events and ephemeral drag previews.
//
// Seq semantics: committed events (library.*, block.*, trip.updated,
// member.changed) increment the per trip counter. Ephemeral messages
// (presence.update, block.preview, block.preview_end, error) reuse the current
// value so a client that skips its own previews never sees a false gap.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/timeutil"
)

const (
	protocolVersion   = 1
	maxClientsPerRoom = 50
	sendBufferSize    = 256
	readLimitBytes    = 4096
	pingInterval      = 25 * time.Second
	pingTimeout       = 60 * time.Second
	presenceGrace     = 5 * time.Second
	previewTTL        = 5 * time.Second
	previewRatePerSec = 20
	writeTimeout      = 10 * time.Second
)

var ErrRoomFull = errors.New("room is full")

// Publisher is what services use to announce committed changes.
type Publisher interface {
	Publish(tripID uuid.UUID, eventType string, origin string, payload any)
	DisconnectUser(tripID uuid.UUID, userID uuid.UUID)
	UpdateRole(tripID uuid.UUID, userID uuid.UUID, role string)
	CloseTrip(tripID uuid.UUID)
}

type Envelope struct {
	V       int    `json:"v"`
	Type    string `json:"type"`
	Seq     int64  `json:"seq"`
	Origin  string `json:"origin,omitempty"`
	Payload any    `json:"payload"`
}

type User struct {
	ID        uuid.UUID
	Name      string
	AvatarURL *string
}

type Viewing struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

type PresenceUser struct {
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url"`
	Viewing   *Viewing  `json:"viewing"`
}

type Hub struct {
	log *logger.LayerLogger

	mu     sync.Mutex
	rooms  map[uuid.UUID]*room
	seqs   map[uuid.UUID]int64
	closed bool
}

type room struct {
	tripID   uuid.UUID
	clients  map[*Client]struct{}
	presence map[uuid.UUID]*presenceEntry
}

type presenceEntry struct {
	user    User
	conns   int
	viewing *Viewing
	timer   *time.Timer
}

func NewHub(appLogger *logger.Logger) *Hub {
	return &Hub{
		log:   appLogger.Layer("realtime.hub"),
		rooms: map[uuid.UUID]*room{},
		seqs:  map[uuid.UUID]int64{},
	}
}

// Client is one WebSocket connection (one browser tab).
type Client struct {
	hub  *Hub
	room *room
	user User
	send chan []byte

	mu          sync.Mutex
	role        string
	previews    map[uuid.UUID]*time.Timer
	cancel      context.CancelFunc
	closeCode   websocket.StatusCode
	closeReason string
	windowStart time.Time
	windowCount int
}

func (c *Client) getRole() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.role
}

func (c *Client) canEdit() bool {
	role := c.getRole()
	return role == "owner" || role == "editor"
}

// closeWith asks the serving goroutine to close the socket with a status.
func (c *Client) closeWith(code websocket.StatusCode, reason string) {
	c.mu.Lock()
	if c.closeCode == 0 {
		c.closeCode = code
		c.closeReason = reason
	}
	cancel := c.cancel
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// allowPreview enforces the 20 messages per second budget.
func (c *Client) allowPreview() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if now.Sub(c.windowStart) >= time.Second {
		c.windowStart = now
		c.windowCount = 0
	}
	if c.windowCount >= previewRatePerSec {
		return false
	}
	c.windowCount++
	return true
}

// Serve registers the connection, pumps messages until it ends and cleans up.
// The caller must already have authenticated the user and checked membership.
func (h *Hub) Serve(ctx context.Context, ws *websocket.Conn, tripID uuid.UUID, user User, role string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	client, err := h.join(tripID, user, role, cancel)
	if err != nil {
		_ = ws.Close(websocket.StatusTryAgainLater, "room full")
		return err
	}

	ws.SetReadLimit(readLimitBytes)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		h.writeLoop(ctx, ws, client)
	}()
	go func() {
		defer wg.Done()
		h.pingLoop(ctx, ws, client)
	}()

	h.readLoop(ctx, ws, client)

	cancel()
	h.leave(client)
	wg.Wait()

	client.mu.Lock()
	code, reason := client.closeCode, client.closeReason
	client.mu.Unlock()

	if code != 0 {
		_ = ws.Close(code, reason)
	} else {
		_ = ws.CloseNow()
	}

	return nil
}

func (h *Hub) writeLoop(ctx context.Context, ws *websocket.Conn, client *Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-client.send:
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := ws.Write(writeCtx, websocket.MessageText, message)
			cancel()
			if err != nil {
				client.closeWith(websocket.StatusInternalError, "write failed")
				return
			}
		}
	}
}

func (h *Hub) pingLoop(ctx context.Context, ws *websocket.Conn, client *Client) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := ws.Ping(pingCtx)
			cancel()
			if err != nil {
				client.closeWith(websocket.StatusGoingAway, "ping timeout")
				return
			}
		}
	}
}

type inbound struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (h *Hub) readLoop(ctx context.Context, ws *websocket.Conn, client *Client) {
	for {
		_, data, err := ws.Read(ctx)
		if err != nil {
			return
		}

		var message inbound
		if err := json.Unmarshal(data, &message); err != nil {
			h.sendError(client, "invalid_message", "message is not valid JSON")
			continue
		}

		switch message.Type {
		case "presence.set":
			h.handlePresenceSet(client, message.Payload)
		case "block.preview":
			h.handlePreview(client, message.Payload)
		case "block.preview_end":
			h.handlePreviewEnd(client, message.Payload)
		default:
			h.sendError(client, "unknown_type", "unknown message type")
		}
	}
}

func (h *Hub) handlePresenceSet(client *Client, raw json.RawMessage) {
	var payload struct {
		Viewing *Viewing `json:"viewing"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		h.sendError(client, "invalid_message", "invalid presence payload")
		return
	}

	if payload.Viewing != nil {
		switch payload.Viewing.Type {
		case "library_item":
			if _, err := uuid.Parse(payload.Viewing.ID); err != nil {
				h.sendError(client, "invalid_message", "viewing.id must be a uuid")
				return
			}
		case "schedule":
		default:
			h.sendError(client, "invalid_message", "viewing.type must be library_item or schedule")
			return
		}
	}

	h.mu.Lock()
	entry := client.room.presence[client.user.ID]
	if entry != nil {
		entry.viewing = payload.Viewing
	}
	h.broadcastPresenceLocked(client.room)
	h.mu.Unlock()
}

type previewPayload struct {
	ID      uuid.UUID `json:"id"`
	StartAt string    `json:"start_at"`
	EndAt   string    `json:"end_at"`
}

func (h *Hub) handlePreview(client *Client, raw json.RawMessage) {
	if !client.canEdit() {
		h.sendError(client, "forbidden", "viewers cannot send previews")
		return
	}
	if !client.allowPreview() {
		h.sendError(client, "rate_limited", "too many preview messages")
		return
	}

	var payload previewPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.ID == uuid.Nil {
		h.sendError(client, "invalid_message", "invalid preview payload")
		return
	}
	start, err := timeutil.ParseUTC(payload.StartAt)
	if err != nil {
		h.sendError(client, "invalid_time", "start_at must be RFC 3339 UTC")
		return
	}
	end, err := timeutil.ParseUTC(payload.EndAt)
	if err != nil || !end.After(start) {
		h.sendError(client, "invalid_time", "end_at must be RFC 3339 UTC after start_at")
		return
	}

	client.mu.Lock()
	if timer, exists := client.previews[payload.ID]; exists {
		timer.Reset(previewTTL)
	} else {
		id := payload.ID
		client.previews[id] = time.AfterFunc(previewTTL, func() { h.expirePreview(client, id) })
	}
	client.mu.Unlock()

	h.mu.Lock()
	h.ephemeralLocked(client.room, "block.preview", "", map[string]any{
		"id":       payload.ID,
		"start_at": start,
		"end_at":   end,
		"user_id":  client.user.ID,
	}, client)
	h.mu.Unlock()
}

func (h *Hub) handlePreviewEnd(client *Client, raw json.RawMessage) {
	if !client.canEdit() {
		h.sendError(client, "forbidden", "viewers cannot send previews")
		return
	}

	var payload previewPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.ID == uuid.Nil {
		h.sendError(client, "invalid_message", "invalid preview payload")
		return
	}

	client.mu.Lock()
	timer, exists := client.previews[payload.ID]
	delete(client.previews, payload.ID)
	client.mu.Unlock()
	if exists {
		timer.Stop()
	}

	h.endPreview(client, payload.ID)
}

func (h *Hub) expirePreview(client *Client, id uuid.UUID) {
	client.mu.Lock()
	_, exists := client.previews[id]
	delete(client.previews, id)
	client.mu.Unlock()

	if exists {
		h.endPreview(client, id)
	}
}

func (h *Hub) endPreview(client *Client, id uuid.UUID) {
	h.mu.Lock()
	h.ephemeralLocked(client.room, "block.preview_end", "", map[string]any{
		"id":      id,
		"user_id": client.user.ID,
	}, client)
	h.mu.Unlock()
}

func (h *Hub) sendError(client *Client, code string, message string) {
	h.mu.Lock()
	seq := h.seqs[client.room.tripID]
	h.mu.Unlock()

	data, err := json.Marshal(Envelope{
		V:       protocolVersion,
		Type:    "error",
		Seq:     seq,
		Payload: map[string]string{"code": code, "message": message},
	})
	if err != nil {
		return
	}
	h.enqueue(client, data)
}

// join registers a client and queues its hello message first, atomically with
// respect to broadcasts, so the snapshot and the following events are ordered.
func (h *Hub) join(tripID uuid.UUID, user User, role string, cancel context.CancelFunc) (*Client, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil, errors.New("hub is shut down")
	}

	r := h.rooms[tripID]
	if r == nil {
		r = &room{
			tripID:   tripID,
			clients:  map[*Client]struct{}{},
			presence: map[uuid.UUID]*presenceEntry{},
		}
		h.rooms[tripID] = r
	}
	if len(r.clients) >= maxClientsPerRoom {
		return nil, ErrRoomFull
	}

	client := &Client{
		hub:      h,
		room:     r,
		user:     user,
		role:     role,
		send:     make(chan []byte, sendBufferSize),
		previews: map[uuid.UUID]*time.Timer{},
		cancel:   cancel,
	}
	r.clients[client] = struct{}{}

	entry := r.presence[user.ID]
	if entry == nil {
		entry = &presenceEntry{user: user}
		r.presence[user.ID] = entry
	}
	if entry.timer != nil {
		entry.timer.Stop()
		entry.timer = nil
	}
	entry.conns++
	entry.user = user

	hello, err := json.Marshal(Envelope{
		V:    protocolVersion,
		Type: "hello",
		Seq:  h.seqs[tripID],
		Payload: map[string]any{
			"user_id":     user.ID,
			"role":        role,
			"server_time": time.Now().UTC(),
			"presence":    presenceSnapshot(r),
		},
	})
	if err == nil {
		client.send <- hello
	}

	h.broadcastPresenceLocked(r)
	return client, nil
}

func (h *Hub) leave(client *Client) {
	client.mu.Lock()
	var pending []uuid.UUID
	for id, timer := range client.previews {
		timer.Stop()
		pending = append(pending, id)
	}
	client.previews = map[uuid.UUID]*time.Timer{}
	client.mu.Unlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	r := client.room
	delete(r.clients, client)

	for _, id := range pending {
		h.ephemeralLocked(r, "block.preview_end", "", map[string]any{"id": id, "user_id": client.user.ID}, client)
	}

	entry := r.presence[client.user.ID]
	if entry != nil {
		entry.conns--
		if entry.conns <= 0 {
			entry.conns = 0
			userID := client.user.ID
			entry.timer = time.AfterFunc(presenceGrace, func() { h.expirePresence(r, userID) })
		}
	}

	h.cleanupRoomLocked(r)
}

// expirePresence drops a user whose last connection stayed gone for the grace
// period, so a page refresh does not make the avatar blink.
func (h *Hub) expirePresence(r *room, userID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry := r.presence[userID]
	if entry == nil || entry.conns > 0 {
		return
	}

	delete(r.presence, userID)
	h.broadcastPresenceLocked(r)
	h.cleanupRoomLocked(r)
}

func (h *Hub) cleanupRoomLocked(r *room) {
	if len(r.clients) == 0 && len(r.presence) == 0 && h.rooms[r.tripID] == r {
		delete(h.rooms, r.tripID)
	}
}

func presenceSnapshot(r *room) []PresenceUser {
	users := make([]PresenceUser, 0, len(r.presence))
	for _, entry := range r.presence {
		users = append(users, PresenceUser{
			UserID:    entry.user.ID,
			Name:      entry.user.Name,
			AvatarURL: entry.user.AvatarURL,
			Viewing:   entry.viewing,
		})
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].Name != users[j].Name {
			return users[i].Name < users[j].Name
		}
		return users[i].UserID.String() < users[j].UserID.String()
	})
	return users
}

func (h *Hub) broadcastPresenceLocked(r *room) {
	h.ephemeralLocked(r, "presence.update", "", map[string]any{"users": presenceSnapshot(r)}, nil)
}

// ephemeralLocked sends without consuming a seq number; except skips the sender.
func (h *Hub) ephemeralLocked(r *room, eventType string, origin string, payload any, except *Client) {
	data, err := json.Marshal(Envelope{
		V:       protocolVersion,
		Type:    eventType,
		Seq:     h.seqs[r.tripID],
		Origin:  origin,
		Payload: payload,
	})
	if err != nil {
		h.log.Error(nil, "marshal", err, "type", eventType)
		return
	}

	for client := range r.clients {
		if client == except {
			continue
		}
		h.enqueue(client, data)
	}
}

// enqueue never blocks: a client whose buffer is full is too slow and is
// closed so it reconnects and fetches a fresh snapshot.
func (h *Hub) enqueue(client *Client, data []byte) {
	select {
	case client.send <- data:
	default:
		client.closeWith(websocket.StatusPolicyViolation, "slow consumer")
	}
}

// Publish announces a committed change to everyone in the trip, sender included.
func (h *Hub) Publish(tripID uuid.UUID, eventType string, origin string, payload any) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.seqs[tripID]++
	seq := h.seqs[tripID]

	r := h.rooms[tripID]
	if r == nil {
		return
	}

	data, err := json.Marshal(Envelope{
		V:       protocolVersion,
		Type:    eventType,
		Seq:     seq,
		Origin:  origin,
		Payload: payload,
	})
	if err != nil {
		h.log.Error(nil, "marshal", err, "type", eventType)
		return
	}

	for client := range r.clients {
		h.enqueue(client, data)
	}
}

func (h *Hub) DisconnectUser(tripID uuid.UUID, userID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r := h.rooms[tripID]
	if r == nil {
		return
	}
	for client := range r.clients {
		if client.user.ID == userID {
			client.closeWith(websocket.StatusPolicyViolation, "removed from trip")
		}
	}
}

func (h *Hub) UpdateRole(tripID uuid.UUID, userID uuid.UUID, role string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r := h.rooms[tripID]
	if r == nil {
		return
	}
	for client := range r.clients {
		if client.user.ID == userID {
			client.mu.Lock()
			client.role = role
			client.mu.Unlock()
		}
	}
}

func (h *Hub) CloseTrip(tripID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r := h.rooms[tripID]
	if r == nil {
		return
	}
	for client := range r.clients {
		client.closeWith(websocket.StatusGoingAway, "trip deleted")
	}
}

// Shutdown closes every connection; used on graceful server stop.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.closed = true
	for _, r := range h.rooms {
		for client := range r.clients {
			client.closeWith(websocket.StatusGoingAway, "server shutting down")
		}
	}
}

// Count returns the number of live connections in a trip (used by tests).
func (h *Hub) Count(tripID uuid.UUID) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	if r := h.rooms[tripID]; r != nil {
		return len(r.clients)
	}
	return 0
}
