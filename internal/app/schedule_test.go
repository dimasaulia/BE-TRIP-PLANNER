package app_test

import (
	"net/http"
	"sync"
	"testing"
)

func TestScheduleRules(t *testing.T) {
	owner, viewer, outsider := newClient(t), newClient(t), newClient(t)
	tripID := owner.newTrip() // 2026-12-14..16 in Asia/Jakarta = 2026-12-13T17:00Z .. 2026-12-16T17:00Z
	owner.invite(tripID, "viewer", viewer)

	first := owner.newBlock(tripID, "2026-12-14T03:00:00Z", "2026-12-14T05:00:00Z", "Pantai").expect(t, http.StatusCreated)
	firstID := first.str("data", "id")
	if first.num("data", "version") != 1 {
		t.Fatalf("new blocks start at version 1: %s", first.Raw)
	}

	t.Run("time must be UTC with Z", func(t *testing.T) {
		owner.newBlock(tripID, "2026-12-14T10:00:00+07:00", "2026-12-14T12:00:00+07:00", "x").
			expectError(t, http.StatusBadRequest, "invalid_time")
		owner.newBlock(tripID, "2026-12-14T10:00:00+00:00", "2026-12-14T12:00:00+00:00", "x").
			expectError(t, http.StatusBadRequest, "invalid_time")
	})

	t.Run("granularity, duration and content", func(t *testing.T) {
		owner.newBlock(tripID, "2026-12-14T08:03:00Z", "2026-12-14T09:00:00Z", "x").
			expectError(t, http.StatusBadRequest, "validation_error")
		owner.newBlock(tripID, "2026-12-14T08:00:30Z", "2026-12-14T09:00:00Z", "x").
			expectError(t, http.StatusBadRequest, "validation_error")
		owner.newBlock(tripID, "2026-12-14T09:00:00Z", "2026-12-14T08:00:00Z", "x").
			expectError(t, http.StatusBadRequest, "validation_error")
		owner.newBlock(tripID, "2026-12-14T06:00:00Z", "2026-12-15T07:00:00Z", "x").
			expectError(t, http.StatusBadRequest, "validation_error")
		owner.do(http.MethodPost, "/trips/"+tripID+"/schedule/blocks", map[string]any{
			"start_at": "2026-12-14T08:00:00Z", "end_at": "2026-12-14T09:00:00Z",
		}).expectError(t, http.StatusBadRequest, "validation_error")
	})

	t.Run("must stay inside the trip dates in the trip timezone", func(t *testing.T) {
		owner.newBlock(tripID, "2026-12-13T16:55:00Z", "2026-12-13T18:00:00Z", "early").
			expectError(t, http.StatusUnprocessableEntity, "outside_trip_range")
		owner.newBlock(tripID, "2026-12-16T16:00:00Z", "2026-12-16T17:05:00Z", "late").
			expectError(t, http.StatusUnprocessableEntity, "outside_trip_range")
		// Exactly the first and last instant of the trip are fine.
		owner.newBlock(tripID, "2026-12-13T17:00:00Z", "2026-12-13T18:00:00Z", "first minute").expect(t, http.StatusCreated)
		owner.newBlock(tripID, "2026-12-16T16:00:00Z", "2026-12-16T17:00:00Z", "last hour").expect(t, http.StatusCreated)
	})

	t.Run("blocks never overlap but may touch", func(t *testing.T) {
		res := owner.newBlock(tripID, "2026-12-14T04:00:00Z", "2026-12-14T06:00:00Z", "clash").
			expectError(t, http.StatusUnprocessableEntity, "block_overlap")
		conflicts, _ := get(res.Body, "error", "details", "conflicts").([]any)
		if len(conflicts) != 1 || conflicts[0] != firstID {
			t.Fatalf("conflicts must name the colliding block: %s", res.Raw)
		}

		touching := owner.newBlock(tripID, "2026-12-14T05:00:00Z", "2026-12-14T07:00:00Z", "touches").expect(t, http.StatusCreated)
		if touching.str("data", "title") != "touches" {
			t.Fatalf("unexpected block: %s", touching.Raw)
		}
	})

	t.Run("version conflicts return the current block", func(t *testing.T) {
		moved := owner.do(http.MethodPatch, "/schedule/blocks/"+firstID, map[string]any{
			"start_at": "2026-12-14T02:00:00Z", "end_at": "2026-12-14T04:00:00Z", "version": 1,
		}).expect(t, http.StatusOK)
		if moved.num("data", "version") != 2 {
			t.Fatalf("version must bump: %s", moved.Raw)
		}

		stale := owner.do(http.MethodPatch, "/schedule/blocks/"+firstID, map[string]any{
			"start_at": "2026-12-14T01:00:00Z", "end_at": "2026-12-14T03:00:00Z", "version": 1,
		}).expectError(t, http.StatusConflict, "version_conflict")
		if get(stale.Body, "error", "details", "current", "version") != float64(2) {
			t.Fatalf("409 must carry the latest block: %s", stale.Raw)
		}

		owner.do(http.MethodPatch, "/schedule/blocks/"+firstID, map[string]any{"start_at": "2026-12-14T01:00:00Z"}).
			expectError(t, http.StatusBadRequest, "validation_error")
	})

	t.Run("moving onto another block is refused", func(t *testing.T) {
		owner.do(http.MethodPatch, "/schedule/blocks/"+firstID, map[string]any{
			"start_at": "2026-12-14T04:30:00Z", "end_at": "2026-12-14T06:30:00Z", "version": 2,
		}).expectError(t, http.StatusUnprocessableEntity, "block_overlap")
	})

	t.Run("list filters by window and carries the item summary", func(t *testing.T) {
		itemID := owner.newItem(tripID, "Tanah Lot")
		withItem := owner.do(http.MethodPost, "/trips/"+tripID+"/schedule/blocks", map[string]any{
			"library_item_id": itemID, "start_at": "2026-12-15T03:00:00Z", "end_at": "2026-12-15T04:00:00Z",
		}).expect(t, http.StatusCreated)
		if withItem.str("data", "library_item", "title") != "Tanah Lot" {
			t.Fatalf("block must embed the item summary: %s", withItem.Raw)
		}

		window := owner.do(http.MethodGet, "/trips/"+tripID+"/schedule?from=2026-12-15T00:00:00Z&to=2026-12-16T00:00:00Z", nil).
			expect(t, http.StatusOK)
		if len(window.items()) != 1 {
			t.Fatalf("expected only the 15th: %s", window.Raw)
		}

		// A library item of another trip cannot be scheduled here.
		foreign := newClient(t)
		foreignItem := foreign.newItem(foreign.newTrip(), "Elsewhere")
		owner.do(http.MethodPost, "/trips/"+tripID+"/schedule/blocks", map[string]any{
			"library_item_id": foreignItem, "start_at": "2026-12-15T06:00:00Z", "end_at": "2026-12-15T07:00:00Z",
		}).expectError(t, http.StatusNotFound, "library_item_not_found")
	})

	t.Run("roles", func(t *testing.T) {
		viewer.do(http.MethodGet, "/trips/"+tripID+"/schedule", nil).expect(t, http.StatusOK)
		viewer.newBlock(tripID, "2026-12-15T08:00:00Z", "2026-12-15T09:00:00Z", "x").expectError(t, http.StatusForbidden, "forbidden")
		viewer.do(http.MethodDelete, "/schedule/blocks/"+firstID, nil).expectError(t, http.StatusForbidden, "forbidden")
		// An outsider cannot tell a real block id from a made up one.
		outsider.do(http.MethodDelete, "/schedule/blocks/"+firstID, nil).expectError(t, http.StatusNotFound, "block_not_found")
	})

	t.Run("trip dates cannot shrink past scheduled blocks", func(t *testing.T) {
		res := owner.do(http.MethodPatch, "/trips/"+tripID, map[string]any{"end_date": "2026-12-14"}).
			expectError(t, http.StatusUnprocessableEntity, "outside_trip_range")
		if ids, _ := get(res.Body, "error", "details", "block_ids").([]any); len(ids) == 0 {
			t.Fatalf("response must name the blocks in the way: %s", res.Raw)
		}
	})

	t.Run("delete", func(t *testing.T) {
		owner.do(http.MethodDelete, "/schedule/blocks/"+firstID, nil).expect(t, http.StatusOK)
		owner.do(http.MethodDelete, "/schedule/blocks/"+firstID, nil).expectError(t, http.StatusNotFound, "block_not_found")
	})
}

// Two requests racing for the same slot must not both win: the exclusion
// constraint is the last line of defence behind the application check.
func TestScheduleParallelOverlap(t *testing.T) {
	owner := newClient(t)
	tripID := owner.newTrip()

	const racers = 8
	statuses := make([]int, racers)

	var wg sync.WaitGroup
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = owner.newBlock(tripID, "2026-12-14T03:00:00Z", "2026-12-14T05:00:00Z", "race").Status
		}()
	}
	wg.Wait()

	created := 0
	for _, status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusUnprocessableEntity:
		default:
			t.Fatalf("unexpected status %d in %v", status, statuses)
		}
	}
	if created != 1 {
		t.Fatalf("exactly one racer must win, got %d in %v", created, statuses)
	}
}

func TestBlockHue(t *testing.T) {
	editor := newClient(t)
	tripID := editor.newTrip()

	created := editor.do(http.MethodPost, "/trips/"+tripID+"/schedule/blocks", map[string]any{
		"title": "Catatan", "hue": 200, "start_at": "2026-12-15T02:00:00Z", "end_at": "2026-12-15T03:00:00Z",
	}).expect(t, http.StatusCreated)
	if created.num("data", "hue") != 200 {
		t.Fatalf("hue = %v", get(created.Body, "data", "hue"))
	}
	blockID := created.str("data", "id")

	editor.do(http.MethodPatch, "/schedule/blocks/"+blockID, map[string]any{"version": 1, "hue": 400}).
		expectError(t, http.StatusBadRequest, "validation_error")

	cleared := editor.do(http.MethodPatch, "/schedule/blocks/"+blockID, map[string]any{"version": 1, "hue": nil}).expect(t, http.StatusOK)
	if get(cleared.Body, "data", "hue") != nil {
		t.Fatal("null hue should clear the colour")
	}
}
