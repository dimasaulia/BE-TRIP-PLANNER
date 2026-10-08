package dto

import (
	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/shared/optional"
	"github.com/open-suite/boilerplate-golang/internal/shared/timeutil"
)

type CreateItemRequest struct {
	Kind          string        `json:"kind"`
	Title         string        `json:"title"`
	DescriptionMD string        `json:"description_md"`
	LocationName  *string       `json:"location_name"`
	Lat           *float64      `json:"lat"`
	Lng           *float64      `json:"lng"`
	LinkURL       *string       `json:"link_url"`
	FixedStartAt  *timeutil.UTC `json:"fixed_start_at"`
	FixedEndAt    *timeutil.UTC `json:"fixed_end_at"`
	Category      *string       `json:"category"`
	// DefaultDurationMin defaults to 60 when omitted.
	DefaultDurationMin *int `json:"default_duration_min"`
	// BannerURL is an http(s) image URL or an image uploaded to this trip.
	BannerURL *string `json:"banner_url"`
}

// UpdateItemRequest uses optional.Value so a field can be left alone, set, or
// cleared with an explicit null.
type UpdateItemRequest struct {
	Version            *int                         `json:"version"`
	Kind               optional.Value[string]       `json:"kind"`
	Title              optional.Value[string]       `json:"title"`
	DescriptionMD      optional.Value[string]       `json:"description_md"`
	LocationName       optional.Value[string]       `json:"location_name"`
	Lat                optional.Value[float64]      `json:"lat"`
	Lng                optional.Value[float64]      `json:"lng"`
	LinkURL            optional.Value[string]       `json:"link_url"`
	FixedStartAt       optional.Value[timeutil.UTC] `json:"fixed_start_at"`
	FixedEndAt         optional.Value[timeutil.UTC] `json:"fixed_end_at"`
	Category           optional.Value[string]       `json:"category"`
	DefaultDurationMin optional.Value[int]          `json:"default_duration_min"`
	BannerURL          optional.Value[string]       `json:"banner_url"`
}

type ExportRequest struct {
	TargetTripID uuid.UUID   `json:"target_trip_id"`
	ItemIDs      []uuid.UUID `json:"item_ids"`
}

type ListResponse struct {
	Items      []entities.LibrarySummary `json:"items"`
	NextCursor *string                   `json:"next_cursor"`
}

type ExportResponse struct {
	Items []entities.LibraryItem `json:"items"`
}
