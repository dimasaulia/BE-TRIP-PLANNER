package entities

import (
	"time"

	"github.com/google/uuid"
)

type LibraryItem struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	TripID        uuid.UUID  `db:"trip_id" json:"trip_id"`
	Kind          string     `db:"kind" json:"kind"`
	Title         string     `db:"title" json:"title"`
	DescriptionMD string     `db:"description_md" json:"description_md"`
	LocationName  *string    `db:"location_name" json:"location_name"`
	Lat           *float64   `db:"lat" json:"lat"`
	Lng           *float64   `db:"lng" json:"lng"`
	LinkURL       *string    `db:"link_url" json:"link_url"`
	FixedStartAt  *time.Time `db:"fixed_start_at" json:"fixed_start_at"`
	FixedEndAt    *time.Time `db:"fixed_end_at" json:"fixed_end_at"`
	// Category is free text; its colour is chosen per browser, never stored.
	Category           *string    `db:"category" json:"category"`
	DefaultDurationMin int        `db:"default_duration_min" json:"default_duration_min"`
	BannerURL          *string    `db:"banner_url" json:"banner_url"`
	Version            int        `db:"version" json:"version"`
	CreatedBy          *uuid.UUID `db:"created_by" json:"created_by"`
	CreatedAt          time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at" json:"updated_at"`
}

// LibrarySummary is the light shape used by lists and realtime events; it
// leaves out the potentially large Markdown description.
type LibrarySummary struct {
	ID                 uuid.UUID  `json:"id"`
	TripID             uuid.UUID  `json:"trip_id"`
	Kind               string     `json:"kind"`
	Title              string     `json:"title"`
	LocationName       *string    `json:"location_name"`
	Lat                *float64   `json:"lat"`
	Lng                *float64   `json:"lng"`
	LinkURL            *string    `json:"link_url"`
	FixedStartAt       *time.Time `json:"fixed_start_at"`
	FixedEndAt         *time.Time `json:"fixed_end_at"`
	Category           *string    `json:"category"`
	DefaultDurationMin int        `json:"default_duration_min"`
	BannerURL          *string    `json:"banner_url"`
	Version            int        `json:"version"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (i LibraryItem) Summary() LibrarySummary {
	return LibrarySummary{
		ID:                 i.ID,
		TripID:             i.TripID,
		Kind:               i.Kind,
		Title:              i.Title,
		LocationName:       i.LocationName,
		Lat:                i.Lat,
		Lng:                i.Lng,
		LinkURL:            i.LinkURL,
		FixedStartAt:       i.FixedStartAt,
		FixedEndAt:         i.FixedEndAt,
		Category:           i.Category,
		DefaultDurationMin: i.DefaultDurationMin,
		BannerURL:          i.BannerURL,
		Version:            i.Version,
		CreatedAt:          i.CreatedAt,
		UpdatedAt:          i.UpdatedAt,
	}
}
