package services

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/shared/validation"
)

const maxDescriptionBytes = 100 * 1024

// validateItem checks a fully merged item, so create and patch share one rule set.
func validateItem(item entities.LibraryItem) error {
	validator := validation.New()

	validator.Check(item.Kind == "destination" || item.Kind == "event", "kind", "must be destination or event")

	title := strings.TrimSpace(item.Title)
	validator.Check(title != "", "title", "is required")
	validator.Check(utf8.RuneCountInString(title) <= 200, "title", "must be at most 200 characters")

	validator.Check(len(item.DescriptionMD) <= maxDescriptionBytes, "description_md", "must be at most 100 KB")

	if item.LocationName != nil {
		validator.Check(utf8.RuneCountInString(*item.LocationName) <= 200, "location_name", "must be at most 200 characters")
	}

	validator.Check((item.Lat == nil) == (item.Lng == nil), "lat", "lat and lng must be provided together")
	if item.Lat != nil {
		validator.Check(*item.Lat >= -90 && *item.Lat <= 90, "lat", "must be between -90 and 90")
	}
	if item.Lng != nil {
		validator.Check(*item.Lng >= -180 && *item.Lng <= 180, "lng", "must be between -180 and 180")
	}

	if item.LinkURL != nil {
		parsed, err := url.Parse(*item.LinkURL)
		validator.Check(
			err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && len(*item.LinkURL) <= 2048,
			"link_url", "must be an http(s) URL",
		)
	}

	if item.Category != nil {
		validator.Check(utf8.RuneCountInString(*item.Category) <= 40, "category", "must be at most 40 characters")
	}
	validator.Check(
		item.DefaultDurationMin >= 30 && item.DefaultDurationMin <= 240 && item.DefaultDurationMin%5 == 0,
		"default_duration_min", "must be between 30 and 240 minutes in steps of 5",
	)

	if item.FixedStartAt != nil && item.FixedEndAt != nil {
		validator.Check(!item.FixedEndAt.Before(*item.FixedStartAt), "fixed_end_at", "must not be before fixed_start_at")
	}

	return validator.Err()
}

func sameTime(a *time.Time, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func sameString(a *string, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameFloat(a *float64, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// summaryChanged says whether a realtime library.updated event is warranted;
// description edits deliberately do not broadcast.
func summaryChanged(before entities.LibraryItem, after entities.LibraryItem) bool {
	return before.Kind != after.Kind ||
		before.Title != after.Title ||
		!sameString(before.LocationName, after.LocationName) ||
		!sameFloat(before.Lat, after.Lat) ||
		!sameFloat(before.Lng, after.Lng) ||
		!sameString(before.LinkURL, after.LinkURL) ||
		!sameTime(before.FixedStartAt, after.FixedStartAt) ||
		!sameTime(before.FixedEndAt, after.FixedEndAt) ||
		!sameString(before.Category, after.Category) ||
		before.DefaultDurationMin != after.DefaultDurationMin ||
		!sameString(before.BannerURL, after.BannerURL)
}

// calendarChanged says whether Google events built from the item are stale.
func calendarChanged(before entities.LibraryItem, after entities.LibraryItem) bool {
	return before.Title != after.Title ||
		before.DescriptionMD != after.DescriptionMD ||
		!sameString(before.LocationName, after.LocationName)
}
