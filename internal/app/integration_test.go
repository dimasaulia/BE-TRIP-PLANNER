package app_test

import (
	"net/http"
	"testing"
)

func TestAuthAndOrigin(t *testing.T) {
	anonymous := &client{t: t, http: http.DefaultClient}

	anonymous.do(http.MethodGet, "/me", nil).expectError(t, http.StatusUnauthorized, "unauthorized")

	user := newClient(t)
	me := user.do(http.MethodGet, "/me", nil).expect(t, http.StatusOK)
	if me.str("data", "user", "email") != user.email {
		t.Fatalf("unexpected profile: %s", me.Raw)
	}
	if me.str("data", "calendar", "status") != "not_connected" {
		t.Fatalf("expected not_connected calendar: %s", me.Raw)
	}

	// A browser origin outside ALLOWED_ORIGINS cannot mutate.
	user.raw(http.MethodPost, "/trips", nil, "application/json", "https://evil.example").
		expectError(t, http.StatusForbidden, "origin_not_allowed")

	user.do(http.MethodPost, "/auth/logout", nil).expect(t, http.StatusOK)
	user.do(http.MethodGet, "/me", nil).expectError(t, http.StatusUnauthorized, "unauthorized")
}

func TestGoogleLoginNotConfigured(t *testing.T) {
	anonymous := &client{t: t, http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	anonymous.do(http.MethodGet, "/auth/google/login", nil).expectError(t, http.StatusServiceUnavailable, "google_not_configured")
}

func TestTripCRUDAndValidation(t *testing.T) {
	owner := newClient(t)

	owner.do(http.MethodPost, "/trips", map[string]any{
		"name": "", "start_date": "2026-12-16", "end_date": "2026-12-14", "timezone": "Mars/Base",
	}).expectError(t, http.StatusBadRequest, "validation_error")

	tripID := owner.newTrip()

	detail := owner.do(http.MethodGet, "/trips/"+tripID, nil).expect(t, http.StatusOK)
	if detail.str("data", "my_role") != "owner" || detail.str("data", "start_date") != "2026-12-14" {
		t.Fatalf("unexpected detail: %s", detail.Raw)
	}
	if members, _ := get(detail.Body, "data", "members").([]any); len(members) != 1 {
		t.Fatalf("expected one member: %s", detail.Raw)
	}

	updated := owner.do(http.MethodPatch, "/trips/"+tripID, map[string]any{"name": "Bali & Ubud"}).expect(t, http.StatusOK)
	if updated.str("data", "name") != "Bali & Ubud" {
		t.Fatalf("not renamed: %s", updated.Raw)
	}

	list := owner.do(http.MethodGet, "/trips", nil).expect(t, http.StatusOK)
	if len(list.items()) != 1 {
		t.Fatalf("expected one trip: %s", list.Raw)
	}

	owner.do(http.MethodDelete, "/trips/"+tripID, nil).expect(t, http.StatusOK)
	owner.do(http.MethodGet, "/trips/"+tripID, nil).expectError(t, http.StatusNotFound, "trip_not_found")
}

func TestRolesAndMembers(t *testing.T) {
	owner, editor, viewer, outsider := newClient(t), newClient(t), newClient(t), newClient(t)
	tripID := owner.newTrip()
	owner.invite(tripID, "editor", editor)
	owner.invite(tripID, "viewer", viewer)

	// Non members cannot learn that the trip exists.
	outsider.do(http.MethodGet, "/trips/"+tripID, nil).expectError(t, http.StatusNotFound, "trip_not_found")
	outsider.do(http.MethodGet, "/trips/"+tripID+"/library", nil).expectError(t, http.StatusNotFound, "trip_not_found")

	// Role matrix on a few representative actions.
	viewer.do(http.MethodGet, "/trips/"+tripID, nil).expect(t, http.StatusOK)
	viewer.do(http.MethodPost, "/trips/"+tripID+"/library", map[string]any{"kind": "event", "title": "x"}).
		expectError(t, http.StatusForbidden, "forbidden")
	viewer.do(http.MethodPost, "/trips/"+tripID+"/invites", map[string]any{"role": "viewer"}).
		expectError(t, http.StatusForbidden, "forbidden")
	editor.do(http.MethodPatch, "/trips/"+tripID, map[string]any{"name": "nope"}).
		expectError(t, http.StatusForbidden, "forbidden")
	editor.do(http.MethodDelete, "/trips/"+tripID, nil).expectError(t, http.StatusForbidden, "forbidden")
	editor.newItem(tripID, "Allowed")

	// Only owners change roles.
	editor.do(http.MethodPatch, "/trips/"+tripID+"/members/"+viewer.userID, map[string]any{"role": "editor"}).
		expectError(t, http.StatusForbidden, "forbidden")
	owner.do(http.MethodPatch, "/trips/"+tripID+"/members/"+viewer.userID, map[string]any{"role": "editor"}).
		expect(t, http.StatusOK)
	viewer.newItem(tripID, "Now an editor")

	// A trip always keeps an owner.
	owner.do(http.MethodDelete, "/trips/"+tripID+"/members/"+owner.userID, nil).
		expectError(t, http.StatusUnprocessableEntity, "last_owner")
	owner.do(http.MethodPatch, "/trips/"+tripID+"/members/"+owner.userID, map[string]any{"role": "viewer"}).
		expectError(t, http.StatusUnprocessableEntity, "last_owner")

	// Members may leave; others may not be removed by non owners.
	editor.do(http.MethodDelete, "/trips/"+tripID+"/members/"+viewer.userID, nil).
		expectError(t, http.StatusForbidden, "forbidden")
	editor.do(http.MethodDelete, "/trips/"+tripID+"/members/"+editor.userID, nil).expect(t, http.StatusOK)
	editor.do(http.MethodGet, "/trips/"+tripID, nil).expectError(t, http.StatusNotFound, "trip_not_found")

	// With a second owner the first one may leave.
	owner.do(http.MethodPatch, "/trips/"+tripID+"/members/"+viewer.userID, map[string]any{"role": "owner"}).expect(t, http.StatusOK)
	owner.do(http.MethodDelete, "/trips/"+tripID+"/members/"+owner.userID, nil).expect(t, http.StatusOK)
}

func TestInvites(t *testing.T) {
	owner, editor, guest, other := newClient(t), newClient(t), newClient(t), newClient(t)
	tripID := owner.newTrip()
	owner.invite(tripID, "editor", editor)

	created := owner.do(http.MethodPost, "/trips/"+tripID+"/invites", map[string]any{"role": "viewer", "max_uses": 1}).
		expect(t, http.StatusCreated)
	token := created.str("data", "token")
	if token == "" || created.str("data", "url") != testOrigin+"/join/"+token {
		t.Fatalf("token and link are shown once: %s", created.Raw)
	}

	// The public preview names the trip and role but needs no login.
	anonymous := &client{t: t, http: http.DefaultClient}
	preview := anonymous.do(http.MethodGet, "/invites/"+token, nil).expect(t, http.StatusOK)
	if preview.str("data", "trip_name") != "Bali" || preview.str("data", "role") != "viewer" {
		t.Fatalf("unexpected preview: %s", preview.Raw)
	}

	guest.do(http.MethodPost, "/invites/"+token+"/accept", nil).expect(t, http.StatusOK)
	// Accepting again is idempotent and does not spend another use.
	again := guest.do(http.MethodPost, "/invites/"+token+"/accept", nil).expect(t, http.StatusOK)
	if get(again.Body, "data", "joined") != false {
		t.Fatalf("second accept must report joined=false: %s", again.Raw)
	}
	// The single use is gone for everyone else.
	other.do(http.MethodPost, "/invites/"+token+"/accept", nil).expectError(t, http.StatusUnprocessableEntity, "invite_exhausted")

	// An existing owner is never downgraded by a viewer invite.
	downgrade := owner.do(http.MethodPost, "/trips/"+tripID+"/invites", map[string]any{"role": "viewer"}).expect(t, http.StatusCreated)
	res := owner.do(http.MethodPost, "/invites/"+downgrade.str("data", "token")+"/accept", nil).expect(t, http.StatusOK)
	if res.str("data", "role") != "owner" {
		t.Fatalf("owner must keep role: %s", res.Raw)
	}

	// Editors revoke only their own invites; owners revoke anything.
	ownerInvite := downgrade.str("data", "id")
	editor.do(http.MethodDelete, "/trips/"+tripID+"/invites/"+ownerInvite, nil).expectError(t, http.StatusForbidden, "forbidden")
	mine := editor.do(http.MethodPost, "/trips/"+tripID+"/invites", map[string]any{"role": "viewer"}).expect(t, http.StatusCreated)
	editor.do(http.MethodDelete, "/trips/"+tripID+"/invites/"+mine.str("data", "id"), nil).expect(t, http.StatusOK)
	other.do(http.MethodPost, "/invites/"+mine.str("data", "token")+"/accept", nil).expectError(t, http.StatusNotFound, "invite_not_found")
	owner.do(http.MethodDelete, "/trips/"+tripID+"/invites/"+ownerInvite, nil).expect(t, http.StatusOK)

	// Viewers cannot be invited as owner, and bad input is rejected.
	owner.do(http.MethodPost, "/trips/"+tripID+"/invites", map[string]any{"role": "owner"}).
		expectError(t, http.StatusBadRequest, "validation_error")
	anonymous.do(http.MethodGet, "/invites/not-a-real-token", nil).expectError(t, http.StatusNotFound, "invite_not_found")

	// Tokens are only ever stored as hashes.
	var stored int
	if err := testDB.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM trip_invites WHERE token_hash = convert_to($1, 'UTF8')`, token).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("raw token must not be stored (count=%d err=%v)", stored, err)
	}
}
