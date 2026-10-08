package services

import (
	"time"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/timeutil"
	"github.com/open-suite/boilerplate-golang/internal/shared/validation"
)

const maxBlockDuration = 24 * time.Hour

// ValidateBlockTimes applies the schedule rules to UTC instants: end after
// start, at most 24 hours, aligned to the granularity in the trip timezone
// (400), and inside the trip dates in that timezone (422).
func ValidateBlockTimes(start time.Time, end time.Time, trip entities.Trip, granularityMin int) error {
	validator := validation.New()
	location := trip.Location()

	validator.Check(end.After(start), "end_at", "must be after start_at")
	if end.After(start) {
		validator.Check(end.Sub(start) <= maxBlockDuration, "end_at", "block can last at most 24 hours")
	}
	validator.Check(aligned(start, location, granularityMin), "start_at", alignMessage(granularityMin))
	validator.Check(aligned(end, location, granularityMin), "end_at", alignMessage(granularityMin))
	if err := validator.Err(); err != nil {
		return err
	}

	rangeStart, rangeEnd, err := timeutil.TripRange(trip.StartDate, trip.EndDate, location)
	if err != nil {
		return err
	}
	if start.Before(rangeStart) || end.After(rangeEnd) {
		return apperror.Unprocessable("outside_trip_range").
			With("range_start", rangeStart).
			With("range_end", rangeEnd).
			With("timezone", trip.Timezone)
	}

	return nil
}

func aligned(instant time.Time, location *time.Location, granularityMin int) bool {
	if granularityMin <= 0 {
		granularityMin = 5
	}
	local := instant.In(location)
	return local.Second() == 0 && local.Nanosecond() == 0 && local.Minute()%granularityMin == 0
}

func alignMessage(granularityMin int) string {
	if granularityMin <= 0 {
		granularityMin = 5
	}
	return "must fall on a multiple of " + itoa(granularityMin) + " minutes with zero seconds"
}
