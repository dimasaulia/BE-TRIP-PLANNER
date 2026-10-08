package services

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/repositories"
)

func TestEventIDIsDeterministicAndGoogleSafe(t *testing.T) {
	sync, block := uuid.New(), uuid.New()

	first, second := EventID(sync, block), EventID(sync, block)
	if first != second {
		t.Fatal("the id must be stable so redelivery cannot create duplicates")
	}
	if first == EventID(sync, uuid.New()) || first == EventID(uuid.New(), block) {
		t.Fatal("different blocks or syncs must get different ids")
	}

	// Google accepts base32hex: lowercase a-v and digits, 5 to 1024 characters.
	if !regexp.MustCompile(`^[a-v0-9]{5,1000}$`).MatchString(first) {
		t.Fatalf("id %q is not valid for Google", first)
	}
}

func ptr(value string) *string { return &value }

func TestBuildEvent(t *testing.T) {
	syncID, tripID, blockID := uuid.New(), uuid.New(), uuid.New()
	start := time.Date(2026, 12, 14, 3, 0, 0, 0, time.UTC)

	base := repositories.BlockData{
		ID: blockID, TripID: tripID, StartAt: start, EndAt: start.Add(2 * time.Hour),
		TripName: "Bali", Timezone: "Asia/Jakarta",
	}

	t.Run("follows the library item", func(t *testing.T) {
		data := base
		data.ItemTitle = ptr("Tanah Lot")
		data.LocationName = ptr("Tabanan")
		data.DescriptionMD = ptr("# Sunset\n\nBring a **jacket**. See [map](https://maps.example/x) ![pic](/api/v1/files/a/b.png)")

		event := BuildEvent(syncID, data, "https://app.example")

		if event.Summary != "Tanah Lot" || event.Location != "Tabanan" || event.TimeZone != "Asia/Jakarta" {
			t.Fatalf("unexpected event: %+v", event)
		}
		if !event.Start.Equal(start) || !event.End.Equal(start.Add(2*time.Hour)) {
			t.Fatalf("times must pass through: %+v", event)
		}
		if event.ID != EventID(syncID, blockID) {
			t.Fatal("the event id must be the deterministic one")
		}
		if !strings.HasPrefix(event.Description, "Sunset Bring a jacket. See map") {
			t.Fatalf("description should be plain text, got %q", event.Description)
		}
		if !strings.HasSuffix(event.Description, "https://app.example/trips/"+tripID.String()) {
			t.Fatalf("description must link back to the trip, got %q", event.Description)
		}
	})

	t.Run("a block title overrides the item title", func(t *testing.T) {
		data := base
		data.Title = ptr("Sunset dinner")
		data.ItemTitle = ptr("Tanah Lot")

		if got := BuildEvent(syncID, data, "https://app.example").Summary; got != "Sunset dinner" {
			t.Fatalf("summary = %q", got)
		}
	})

	t.Run("description is cut at 300 characters", func(t *testing.T) {
		data := base
		data.Title = ptr("x")
		data.DescriptionMD = ptr(strings.Repeat("abcdefghij", 100))

		event := BuildEvent(syncID, data, "https://app.example")
		body := strings.SplitN(event.Description, "\n\n", 2)[0]
		if len([]rune(body)) != 300 {
			t.Fatalf("expected 300 characters before the link, got %d", len([]rune(body)))
		}
	})
}

func TestContentHashDetectsRealChangesOnly(t *testing.T) {
	syncID, blockID := uuid.New(), uuid.New()
	start := time.Date(2026, 12, 14, 3, 0, 0, 0, time.UTC)
	data := repositories.BlockData{ID: blockID, TripID: uuid.New(), Title: ptr("A"), StartAt: start, EndAt: start.Add(time.Hour), Timezone: "UTC"}

	original := ContentHash(BuildEvent(syncID, data, "https://app.example"))
	if original != ContentHash(BuildEvent(syncID, data, "https://app.example")) {
		t.Fatal("hash must be stable for identical content")
	}

	moved := data
	moved.StartAt = start.Add(5 * time.Minute)
	if original == ContentHash(BuildEvent(syncID, moved, "https://app.example")) {
		t.Fatal("moving a block must change the hash")
	}

	renamed := data
	renamed.Title = ptr("B")
	if original == ContentHash(BuildEvent(syncID, renamed, "https://app.example")) {
		t.Fatal("renaming a block must change the hash")
	}
}

func TestPlainText(t *testing.T) {
	tests := map[string]string{
		"# Title\n\nSome *emphasis* and `code`": "Title Some emphasis and code",
		"- [ ] pack bags\n- [x] book hotel":     "pack bags book hotel",
		"![photo](/img.png) caption":            "caption",
		"[link text](https://example.com)":      "link text",
		"```go\nfmt.Println()\n```\nafter":      "after",
		"<script>alert(1)</script>safe":         "alert(1) safe",
		"1. first\n2. second":                   "first second",
		"> quoted":                              "quoted",
		"":                                      "",
	}

	for input, want := range tests {
		if got := PlainText(input, 300); got != want {
			t.Errorf("PlainText(%q) = %q, want %q", input, got, want)
		}
	}
}
