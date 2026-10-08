package services

import (
	"testing"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

func utc(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

// Jakarta is UTC+7, so 2026-12-14 spans 2026-12-13T17:00Z to 2026-12-14T17:00Z.
var jakartaTrip = entities.Trip{StartDate: "2026-12-14", EndDate: "2026-12-16", Timezone: "Asia/Jakarta"}

func TestValidateBlockTimes(t *testing.T) {
	tests := []struct {
		name       string
		start, end string
		trip       entities.Trip
		granular   int
		status     int // 0 means valid
		code       string
	}{
		{"valid", "2026-12-14T03:00:00Z", "2026-12-14T05:00:00Z", jakartaTrip, 5, 0, ""},
		{"first instant of the trip", "2026-12-13T17:00:00Z", "2026-12-13T18:00:00Z", jakartaTrip, 5, 0, ""},
		{"ends exactly at the last instant", "2026-12-16T16:00:00Z", "2026-12-16T17:00:00Z", jakartaTrip, 5, 0, ""},
		{"starts before the trip in Jakarta", "2026-12-13T16:55:00Z", "2026-12-13T18:00:00Z", jakartaTrip, 5, 422, "outside_trip_range"},
		{"runs past the last day", "2026-12-16T16:00:00Z", "2026-12-16T17:05:00Z", jakartaTrip, 5, 422, "outside_trip_range"},
		{"UTC date is inside but Jakarta date is not", "2026-12-13T20:00:00Z", "2026-12-13T21:00:00Z", entities.Trip{StartDate: "2026-12-14", EndDate: "2026-12-14", Timezone: "Asia/Jakarta"}, 5, 0, ""},
		{"end before start", "2026-12-14T05:00:00Z", "2026-12-14T03:00:00Z", jakartaTrip, 5, 400, "validation_error"},
		{"empty block", "2026-12-14T05:00:00Z", "2026-12-14T05:00:00Z", jakartaTrip, 5, 400, "validation_error"},
		{"not on a 5 minute mark", "2026-12-14T03:03:00Z", "2026-12-14T05:00:00Z", jakartaTrip, 5, 400, "validation_error"},
		{"seconds are not allowed", "2026-12-14T03:00:30Z", "2026-12-14T05:00:00Z", jakartaTrip, 5, 400, "validation_error"},
		{"exactly 24 hours", "2026-12-14T00:00:00Z", "2026-12-15T00:00:00Z", jakartaTrip, 5, 0, ""},
		{"more than 24 hours", "2026-12-14T00:00:00Z", "2026-12-15T00:05:00Z", jakartaTrip, 5, 400, "validation_error"},
		{"granularity follows config", "2026-12-14T03:10:00Z", "2026-12-14T05:00:00Z", jakartaTrip, 15, 400, "validation_error"},
		{"granularity 15 accepts quarter hours", "2026-12-14T03:15:00Z", "2026-12-14T05:00:00Z", jakartaTrip, 15, 0, ""},
		// Half hour zones: the grid is local time, so 03:30Z is :00 in Mumbai-like +5:30.
		{"granularity is checked in the trip timezone", "2026-12-14T03:30:00Z", "2026-12-14T05:30:00Z", entities.Trip{StartDate: "2026-12-14", EndDate: "2026-12-14", Timezone: "Asia/Kolkata"}, 60, 0, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateBlockTimes(utc(t, test.start), utc(t, test.end), test.trip, test.granular)

			if test.status == 0 {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}

			appErr, ok := apperror.As(err)
			if !ok || appErr.Status != test.status || appErr.Code != test.code {
				t.Fatalf("expected %d %s, got %v", test.status, test.code, err)
			}
		})
	}
}
