package timeutil

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

func TestParseUTCOnlyAcceptsZ(t *testing.T) {
	parsed, err := ParseUTC("2026-12-14T03:00:00Z")
	if err != nil || !parsed.Equal(time.Date(2026, 12, 14, 3, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected a valid UTC time, got %v %v", parsed, err)
	}

	for _, input := range []string{
		"2026-12-14T10:00:00+07:00",
		"2026-12-14T03:00:00+00:00",
		"2026-12-14T03:00:00",
		"2026-12-14",
		"nonsense",
		"",
	} {
		_, err := ParseUTC(input)
		appErr, ok := apperror.As(err)
		if !ok || appErr.Code != "invalid_time" || appErr.Status != 400 {
			t.Errorf("%q must be rejected with 400 invalid_time, got %v", input, err)
		}
	}
}

func TestUTCUnmarshalSurfacesInvalidTime(t *testing.T) {
	var body struct {
		At UTC `json:"at"`
	}

	if err := json.Unmarshal([]byte(`{"at":"2026-12-14T03:00:00Z"}`), &body); err != nil {
		t.Fatal(err)
	}

	err := json.Unmarshal([]byte(`{"at":"2026-12-14T03:00:00+07:00"}`), &body)
	if appErr, ok := apperror.As(err); !ok || appErr.Code != "invalid_time" {
		t.Fatalf("decoder must surface invalid_time, got %v", err)
	}

	if err := json.Unmarshal([]byte(`{"at":12345}`), &body); err == nil {
		t.Fatal("a non string time must be rejected")
	}
}

func TestTripRangeUsesTheTripTimezone(t *testing.T) {
	jakarta, _ := time.LoadLocation("Asia/Jakarta")

	start, end, err := TripRange("2026-12-14", "2026-12-16", jakarta)
	if err != nil {
		t.Fatal(err)
	}

	if want := time.Date(2026, 12, 13, 17, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Errorf("start = %v, want %v", start, want)
	}
	// The end is exclusive: the end of the 16th is the start of the 17th local.
	if want := time.Date(2026, 12, 16, 17, 0, 0, 0, time.UTC); !end.Equal(want) {
		t.Errorf("end = %v, want %v", end, want)
	}
}
