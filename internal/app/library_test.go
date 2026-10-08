package app_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/platform/storage"
	"github.com/open-suite/boilerplate-golang/internal/shared/attachments"
)

func pngBytes(t *testing.T, width int, height int) []byte {
	t.Helper()

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	canvas.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// upload sends a multipart body with the given file bytes under field "file".
func (c *client) upload(tripID string, filename string, content []byte) *response {
	c.t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	header.Set("Content-Type", "application/octet-stream")
	part, _ := writer.CreatePart(header)
	_, _ = part.Write(content)
	_ = writer.Close()

	return c.raw(http.MethodPost, "/trips/"+tripID+"/uploads", &body, writer.FormDataContentType(), testOrigin)
}

func (c *client) fetch(path string) (int, string, []byte) {
	c.t.Helper()

	request, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
	result, err := c.http.Do(request)
	if err != nil {
		c.t.Fatal(err)
	}
	defer result.Body.Close()

	data, _ := io.ReadAll(result.Body)
	return result.StatusCode, result.Header.Get("Content-Type"), data
}

func TestLibraryCRUDAndVersions(t *testing.T) {
	editor, viewer := newClient(t), newClient(t)
	tripID := editor.newTrip()
	editor.invite(tripID, "viewer", viewer)

	created := editor.do(http.MethodPost, "/trips/"+tripID+"/library", map[string]any{
		"kind": "event", "title": "Konser", "description_md": "# Konser\n- [ ] tiket",
		"location_name": "GBK", "lat": -6.2, "lng": 106.8, "link_url": "https://example.com/konser",
		"fixed_start_at": "2026-12-15T12:00:00Z", "fixed_end_at": "2026-12-15T15:00:00Z",
	}).expect(t, http.StatusCreated)
	itemID := created.str("data", "id")

	t.Run("validation", func(t *testing.T) {
		for name, body := range map[string]map[string]any{
			"bad kind":     {"kind": "museum", "title": "x"},
			"blank title":  {"kind": "event", "title": "  "},
			"lone lat":     {"kind": "event", "title": "x", "lat": 1.0},
			"bad url":      {"kind": "event", "title": "x", "link_url": "javascript:alert(1)"},
			"fixed order":  {"kind": "event", "title": "x", "fixed_start_at": "2026-12-15T12:00:00Z", "fixed_end_at": "2026-12-15T11:00:00Z"},
			"too big desc": {"kind": "event", "title": "x", "description_md": strings.Repeat("a", 100*1024+1)},
		} {
			editor.do(http.MethodPost, "/trips/"+tripID+"/library", body).
				expectError(t, http.StatusBadRequest, "validation_error")
			_ = name
		}
		editor.do(http.MethodPost, "/trips/"+tripID+"/library", map[string]any{
			"kind": "event", "title": "x", "fixed_start_at": "2026-12-15T19:00:00+07:00",
		}).expectError(t, http.StatusBadRequest, "invalid_time")
	})

	t.Run("outsiders cannot probe item ids", func(t *testing.T) {
		outsider := newClient(t)
		outsider.do(http.MethodGet, "/library/"+itemID, nil).expectError(t, http.StatusNotFound, "library_item_not_found")
		outsider.do(http.MethodGet, "/library/11111111-1111-1111-1111-111111111111", nil).expectError(t, http.StatusNotFound, "library_item_not_found")
	})

	t.Run("detail has markdown, list does not", func(t *testing.T) {
		detail := viewer.do(http.MethodGet, "/library/"+itemID, nil).expect(t, http.StatusOK)
		if !strings.Contains(detail.str("data", "description_md"), "tiket") {
			t.Fatalf("detail must carry the Markdown: %s", detail.Raw)
		}

		list := viewer.do(http.MethodGet, "/trips/"+tripID+"/library", nil).expect(t, http.StatusOK)
		if len(list.items()) != 1 || get(list.items()[0], "description_md") != nil {
			t.Fatalf("list must stay light: %s", list.Raw)
		}
	})

	t.Run("patch with version, null clears a field", func(t *testing.T) {
		updated := editor.do(http.MethodPatch, "/library/"+itemID, map[string]any{
			"version": 1, "description_md": "# Konser\nBawa KTP", "location_name": nil,
		}).expect(t, http.StatusOK)
		if updated.num("data", "version") != 2 || updated.str("data", "location_name") != "" {
			t.Fatalf("unexpected update: %s", updated.Raw)
		}
		if get(updated.Body, "data", "title") != "Konser" {
			t.Fatalf("untouched fields must survive: %s", updated.Raw)
		}

		editor.do(http.MethodPatch, "/library/"+itemID, map[string]any{"version": 1, "title": "Stale"}).
			expectError(t, http.StatusConflict, "version_conflict")
		editor.do(http.MethodPatch, "/library/"+itemID, map[string]any{"title": "No version"}).
			expectError(t, http.StatusBadRequest, "validation_error")
		viewer.do(http.MethodPatch, "/library/"+itemID, map[string]any{"version": 2, "title": "x"}).
			expectError(t, http.StatusForbidden, "forbidden")
	})

	t.Run("filter and cursor pagination", func(t *testing.T) {
		editor.newItem(tripID, "Pantai Kuta")
		editor.newItem(tripID, "Pantai Sanur")
		editor.newItem(tripID, "100% Sate_Lilit")

		pantai := editor.do(http.MethodGet, "/trips/"+tripID+"/library?q=pantai", nil).expect(t, http.StatusOK)
		if len(pantai.items()) != 2 {
			t.Fatalf("search must match titles: %s", pantai.Raw)
		}
		// LIKE wildcards in the query are literal.
		literal := editor.do(http.MethodGet, "/trips/"+tripID+"/library?q=100%25", nil).expect(t, http.StatusOK)
		if len(literal.items()) != 1 {
			t.Fatalf("percent must be literal: %s", literal.Raw)
		}
		kinds := editor.do(http.MethodGet, "/trips/"+tripID+"/library?kind=event", nil).expect(t, http.StatusOK)
		if len(kinds.items()) != 1 {
			t.Fatalf("kind filter: %s", kinds.Raw)
		}

		seen := map[string]bool{}
		cursor := ""
		for page := 0; page < 5; page++ {
			path := "/trips/" + tripID + "/library?limit=2"
			if cursor != "" {
				path += "&cursor=" + cursor
			}
			res := editor.do(http.MethodGet, path, nil).expect(t, http.StatusOK)
			for _, item := range res.items() {
				id := get(item, "id").(string)
				if seen[id] {
					t.Fatalf("item %s repeated across pages", id)
				}
				seen[id] = true
			}
			next, _ := get(res.Body, "data", "next_cursor").(string)
			if next == "" {
				break
			}
			cursor = next
		}
		if len(seen) != 4 {
			t.Fatalf("pagination must visit all 4 items, saw %d", len(seen))
		}
		editor.do(http.MethodGet, "/trips/"+tripID+"/library?cursor=garbage", nil).
			expectError(t, http.StatusBadRequest, "invalid_cursor")
	})
}

func TestLibraryDeleteInUse(t *testing.T) {
	owner := newClient(t)
	tripID := owner.newTrip()
	itemID := owner.newItem(tripID, "Tanah Lot")

	block := owner.do(http.MethodPost, "/trips/"+tripID+"/schedule/blocks", map[string]any{
		"library_item_id": itemID, "start_at": "2026-12-14T03:00:00Z", "end_at": "2026-12-14T05:00:00Z",
	}).expect(t, http.StatusCreated)

	res := owner.do(http.MethodDelete, "/library/"+itemID, nil).expectError(t, http.StatusConflict, "library_item_in_use")
	if res.num("error", "details", "block_count") != 1 {
		t.Fatalf("response should count the blocks: %s", res.Raw)
	}

	// force=true removes the blocks and the item together.
	owner.do(http.MethodDelete, "/library/"+itemID+"?force=true", nil).expect(t, http.StatusOK)
	owner.do(http.MethodGet, "/library/"+itemID, nil).expectError(t, http.StatusNotFound, "library_item_not_found")
	owner.do(http.MethodDelete, "/schedule/blocks/"+block.str("data", "id"), nil).expectError(t, http.StatusNotFound, "block_not_found")

	// An unused item deletes without force.
	free := owner.newItem(tripID, "Free")
	owner.do(http.MethodDelete, "/library/"+free, nil).expect(t, http.StatusOK)
}

func TestUploadRules(t *testing.T) {
	editor, viewer, outsider := newClient(t), newClient(t), newClient(t)
	tripID := editor.newTrip()
	editor.invite(tripID, "viewer", viewer)

	ok := editor.upload(tripID, "bali.png", pngBytes(t, 40, 30)).expect(t, http.StatusCreated)
	if ok.num("data", "width") != 40 || ok.num("data", "height") != 30 {
		t.Fatalf("dimensions come from the image: %s", ok.Raw)
	}
	url := ok.str("data", "url")
	if !strings.HasPrefix(url, "/api/v1/files/"+tripID+"/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("unexpected url %q", url)
	}

	t.Run("served to members only, cached forever", func(t *testing.T) {
		status, contentType, data := viewer.fetch(url)
		if status != http.StatusOK || contentType != "image/png" || len(data) == 0 {
			t.Fatalf("viewer must read the image: %d %s", status, contentType)
		}
		if status, _, _ := outsider.fetch(url); status != http.StatusNotFound {
			t.Fatalf("outsiders must get 404, got %d", status)
		}

		request, _ := http.NewRequest(http.MethodGet, server.URL+url, nil)
		result, err := viewer.http.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer result.Body.Close()
		if result.Header.Get("Cache-Control") != "private, max-age=31536000, immutable" {
			t.Fatalf("cache header: %q", result.Header.Get("Cache-Control"))
		}
	})

	t.Run("content decides the type, not the filename", func(t *testing.T) {
		editor.upload(tripID, "notes.png", []byte("just some text, not an image")).
			expectError(t, http.StatusUnsupportedMediaType, "unsupported_image_type")
		editor.upload(tripID, "empty.png", nil).expectError(t, http.StatusBadRequest, "invalid_image")
	})

	t.Run("size and dimension limits", func(t *testing.T) {
		big := append(pngBytes(t, 10, 10), bytes.Repeat([]byte{0}, 300_000)...)
		editor.upload(tripID, "big.png", big).expectError(t, http.StatusRequestEntityTooLarge, "payload_too_large")
		editor.upload(tripID, "huge.png", pngBytes(t, 4097, 8)).expectError(t, http.StatusUnprocessableEntity, "image_dimensions_exceeded")
	})

	t.Run("roles", func(t *testing.T) {
		viewer.upload(tripID, "a.png", pngBytes(t, 4, 4)).expectError(t, http.StatusForbidden, "forbidden")
		outsider.upload(tripID, "a.png", pngBytes(t, 4, 4)).expectError(t, http.StatusNotFound, "trip_not_found")
	})

	t.Run("path traversal is not a file", func(t *testing.T) {
		if status, _, _ := editor.fetch("/api/v1/files/" + tripID + "/..%2F..%2Fetc%2Fpasswd"); status != http.StatusNotFound && status != http.StatusBadRequest {
			t.Fatalf("traversal must fail, got %d", status)
		}
	})
}

func TestLibraryExport(t *testing.T) {
	owner, otherEditor, viewerOnTarget := newClient(t), newClient(t), newClient(t)
	source := owner.newTrip()
	target := owner.newTrip()
	foreign := otherEditor.newTrip()
	otherEditor.invite(foreign, "viewer", owner)
	owner.invite(target, "viewer", viewerOnTarget)

	image := owner.upload(source, "a.png", pngBytes(t, 20, 20)).expect(t, http.StatusCreated)
	imageURL := image.str("data", "url")

	itemID := owner.do(http.MethodPost, "/trips/"+source+"/library", map[string]any{
		"kind": "destination", "title": "Uluwatu", "description_md": "Sunset\n\n![view](" + imageURL + ")",
	}).expect(t, http.StatusCreated).str("data", "id")
	owner.newItem(source, "Second")

	t.Run("needs editor on both trips", func(t *testing.T) {
		owner.do(http.MethodPost, "/trips/"+source+"/library/export", map[string]any{
			"target_trip_id": foreign, "item_ids": []string{itemID},
		}).expectError(t, http.StatusForbidden, "forbidden")

		viewerOnTarget.do(http.MethodPost, "/trips/"+target+"/library/export", map[string]any{
			"target_trip_id": source, "item_ids": []string{itemID},
		}).expectError(t, http.StatusForbidden, "forbidden")

		owner.do(http.MethodPost, "/trips/"+source+"/library/export", map[string]any{
			"target_trip_id": source, "item_ids": []string{itemID},
		}).expectError(t, http.StatusBadRequest, "validation_error")
	})

	t.Run("all or nothing", func(t *testing.T) {
		owner.do(http.MethodPost, "/trips/"+source+"/library/export", map[string]any{
			"target_trip_id": target, "item_ids": []string{itemID, "11111111-1111-1111-1111-111111111111"},
		}).expectError(t, http.StatusNotFound, "library_item_not_found")

		list := owner.do(http.MethodGet, "/trips/"+target+"/library", nil).expect(t, http.StatusOK)
		if len(list.items()) != 0 {
			t.Fatalf("a failed export must leave nothing behind: %s", list.Raw)
		}
	})

	t.Run("copies items and rewrites image urls", func(t *testing.T) {
		res := owner.do(http.MethodPost, "/trips/"+source+"/library/export", map[string]any{
			"target_trip_id": target, "item_ids": []string{itemID},
		}).expect(t, http.StatusCreated)

		copies := res.items()
		if len(copies) != 1 {
			t.Fatalf("expected one copy: %s", res.Raw)
		}
		copyID := get(copies[0], "id").(string)
		if copyID == itemID || get(copies[0], "trip_id") != target {
			t.Fatalf("the copy is a new item in the target trip: %s", res.Raw)
		}

		description, _ := get(copies[0], "description_md").(string)
		if strings.Contains(description, "/files/"+source+"/") || !strings.Contains(description, "/files/"+target+"/") {
			t.Fatalf("image url must point into the target trip: %q", description)
		}

		// The copied file is readable by the target's members, and independent of the original.
		start := strings.Index(description, "/api/v1/files/")
		copiedURL := description[start : strings.Index(description[start:], ")")+start]
		if status, _, data := viewerOnTarget.fetch(copiedURL); status != http.StatusOK || len(data) == 0 {
			t.Fatalf("copied image must be served from the target trip: %d", status)
		}

		// Items are independent copies: editing one leaves the other alone.
		owner.do(http.MethodPatch, "/library/"+copyID, map[string]any{"version": 1, "title": "Renamed"}).expect(t, http.StatusOK)
		if original := owner.do(http.MethodGet, "/library/"+itemID, nil).expect(t, http.StatusOK); original.str("data", "title") != "Uluwatu" {
			t.Fatalf("original changed: %s", original.Raw)
		}
	})
}

func TestTripBanner(t *testing.T) {
	owner, editor, other := newClient(t), newClient(t), newClient(t)
	tripID := owner.newTrip()
	owner.invite(tripID, "editor", editor)
	foreignTrip := other.newTrip()

	t.Run("external image url at creation", func(t *testing.T) {
		res := owner.do(http.MethodPost, "/trips", map[string]any{
			"name": "With banner", "start_date": "2026-12-14", "end_date": "2026-12-16",
			"timezone": "Asia/Jakarta", "banner_url": "https://images.example.com/bali.jpg",
		}).expect(t, http.StatusCreated)
		if res.str("data", "banner_url") != "https://images.example.com/bali.jpg" {
			t.Fatalf("banner must be stored: %s", res.Raw)
		}
		if owner.do(http.MethodGet, "/trips/"+tripID, nil).expect(t, http.StatusOK).Body["data"].(map[string]any)["banner_url"] != nil {
			t.Fatal("a trip without banner reports null")
		}
	})

	t.Run("upload, then set the returned url", func(t *testing.T) {
		url := owner.upload(tripID, "banner.png", pngBytes(t, 30, 10)).expect(t, http.StatusCreated).str("data", "url")

		set := owner.do(http.MethodPatch, "/trips/"+tripID, map[string]any{"banner_url": url}).expect(t, http.StatusOK)
		if set.str("data", "banner_url") != url {
			t.Fatalf("banner not set: %s", set.Raw)
		}
		if status, _, _ := editor.fetch(url); status != http.StatusOK {
			t.Fatalf("members must be able to load the banner, got %d", status)
		}
		if got := owner.do(http.MethodGet, "/trips", nil).expect(t, http.StatusOK); !strings.Contains(string(got.Raw), url) {
			t.Fatalf("list must carry the banner: %s", got.Raw)
		}

		// Other fields keep the banner; null removes it.
		owner.do(http.MethodPatch, "/trips/"+tripID, map[string]any{"name": "Renamed"}).expect(t, http.StatusOK)
		if owner.do(http.MethodGet, "/trips/"+tripID, nil).str("data", "banner_url") != url {
			t.Fatal("an unrelated patch must not clear the banner")
		}
		cleared := owner.do(http.MethodPatch, "/trips/"+tripID, map[string]any{"banner_url": nil}).expect(t, http.StatusOK)
		if get(cleared.Body, "data", "banner_url") != nil {
			t.Fatalf("null must remove the banner: %s", cleared.Raw)
		}
	})

	t.Run("rejects what is not an image url of this trip", func(t *testing.T) {
		foreignURL := other.upload(foreignTrip, "x.png", pngBytes(t, 5, 5)).expect(t, http.StatusCreated).str("data", "url")

		for _, bad := range []string{
			"javascript:alert(1)", "ftp://x.example/a.png", "not a url", "",
			foreignURL, // another trip's upload
			"/api/v1/files/" + tripID + "/11111111-1111-4111-8111-111111111111.png", // never uploaded
		} {
			owner.do(http.MethodPatch, "/trips/"+tripID, map[string]any{"banner_url": bad}).
				expectError(t, http.StatusBadRequest, "validation_error")
		}
	})

	t.Run("owner only", func(t *testing.T) {
		editor.do(http.MethodPatch, "/trips/"+tripID, map[string]any{"banner_url": "https://images.example.com/x.jpg"}).
			expectError(t, http.StatusForbidden, "forbidden")
	})

	t.Run("cleanup keeps an image the banner uses", func(t *testing.T) {
		url := owner.upload(tripID, "keep.png", pngBytes(t, 8, 8)).expect(t, http.StatusCreated).str("data", "url")
		owner.do(http.MethodPatch, "/trips/"+tripID, map[string]any{"banner_url": url}).expect(t, http.StatusOK)

		if _, err := testDB.Pool.Exec(t.Context(), `UPDATE attachments SET created_at = NOW() - INTERVAL '30 days' WHERE trip_id = $1`, tripID); err != nil {
			t.Fatal(err)
		}
		service := attachments.NewService(testDB, storage.New(testCfg), testCfg, testLog)
		if _, err := service.CleanupOrphans(t.Context(), 7*24*time.Hour); err != nil {
			t.Fatal(err)
		}
		if status, _, _ := owner.fetch(url); status != http.StatusOK {
			t.Fatalf("the banner image must survive cleanup, got %d", status)
		}
	})
}

func TestLibraryDesignFields(t *testing.T) {
	editor := newClient(t)
	tripID := editor.newTrip()

	created := editor.do(http.MethodPost, "/trips/"+tripID+"/library", map[string]any{
		"kind": "destination", "title": "Pantai", "category": "  Kuliner   malam ",
	}).expect(t, http.StatusCreated)
	itemID := created.str("data", "id")
	if got := created.str("data", "category"); got != "Kuliner malam" {
		t.Fatalf("category = %q, want normalised label", got)
	}
	if got := created.num("data", "default_duration_min"); got != 60 {
		t.Fatalf("default_duration_min = %v, want 60", got)
	}

	for name, body := range map[string]map[string]any{
		"short duration":  {"kind": "event", "title": "x", "default_duration_min": 20},
		"odd duration":    {"kind": "event", "title": "x", "default_duration_min": 33},
		"long category":   {"kind": "event", "title": "x", "category": strings.Repeat("a", 41)},
		"relative banner": {"kind": "event", "title": "x", "banner_url": "/etc/passwd"},
		"foreign upload":  {"kind": "event", "title": "x", "banner_url": "/api/v1/files/" + tripID + "/00000000-0000-0000-0000-000000000000.png"},
	} {
		t.Run(name, func(t *testing.T) {
			editor.do(http.MethodPost, "/trips/"+tripID+"/library", body).expectError(t, http.StatusBadRequest, "validation_error")
		})
	}

	upload := editor.upload(tripID, "banner.png", pngBytes(t, 4, 3)).expect(t, http.StatusCreated)
	bannerURL := upload.str("data", "url")

	updated := editor.do(http.MethodPatch, "/library/"+itemID, map[string]any{
		"version": 1, "category": "", "default_duration_min": 120, "banner_url": bannerURL,
	}).expect(t, http.StatusOK)
	if get(updated.Body, "data", "category") != nil {
		t.Fatal("blank category should clear the field")
	}
	if updated.num("data", "default_duration_min") != 120 || updated.str("data", "banner_url") != bannerURL {
		t.Fatalf("unexpected update result: %v", updated.Body)
	}

	list := editor.do(http.MethodGet, "/trips/"+tripID+"/library", nil).expect(t, http.StatusOK)
	first := list.items()[0].(map[string]any)
	if first["banner_url"] != bannerURL || first["default_duration_min"].(float64) != 120 {
		t.Fatalf("list summary misses design fields: %v", first)
	}

	editor.do(http.MethodPatch, "/library/"+itemID, map[string]any{"version": 2, "category": "Pantai"}).expect(t, http.StatusOK)
	block := editor.do(http.MethodPost, "/trips/"+tripID+"/schedule/blocks", map[string]any{
		"library_item_id": itemID, "start_at": "2026-12-15T02:00:00Z", "end_at": "2026-12-15T03:00:00Z",
	}).expect(t, http.StatusCreated)
	if got := block.str("data", "library_item", "category"); got != "Pantai" {
		t.Fatalf("block item summary category = %q", got)
	}
}
