package timeutil

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

const DateLayout = "2006-01-02"

// UTC is a request time that must be RFC 3339 and end in `Z`; any other offset
// is rejected with 400 invalid_time as the API contract demands.
type UTC struct {
	time.Time
}

func (u *UTC) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return apperror.BadRequest("invalid_time")
	}

	parsed, err := ParseUTC(raw)
	if err != nil {
		return err
	}

	u.Time = parsed
	return nil
}

func ParseUTC(raw string) (time.Time, error) {
	if !strings.HasSuffix(raw, "Z") {
		return time.Time{}, apperror.BadRequest("invalid_time").With("value", raw)
	}

	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, apperror.BadRequest("invalid_time").With("value", raw)
	}

	return parsed.UTC(), nil
}

// TripRange converts the inclusive trip dates into the half open UTC interval
// [start, end) they cover in the trip timezone.
func TripRange(startDate string, endDate string, location *time.Location) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation(DateLayout, startDate, location)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := time.ParseInLocation(DateLayout, endDate, location)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	return start.UTC(), end.AddDate(0, 0, 1).UTC(), nil
}
