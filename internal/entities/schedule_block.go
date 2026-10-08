package entities

import (
	"time"

	"github.com/google/uuid"
)

type ScheduleBlock struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	TripID        uuid.UUID  `db:"trip_id" json:"trip_id"`
	LibraryItemID *uuid.UUID `db:"library_item_id" json:"library_item_id"`
	Title         *string    `db:"title" json:"title"`
	Note          string     `db:"note" json:"note"`
	Hue           *int       `db:"hue" json:"hue"`
	StartAt       time.Time  `db:"start_at" json:"start_at"`
	EndAt         time.Time  `db:"end_at" json:"end_at"`
	CreatedBy     *uuid.UUID `db:"created_by" json:"created_by"`
	UpdatedBy     *uuid.UUID `db:"updated_by" json:"updated_by"`
	Version       int        `db:"version" json:"version"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`

	// Joined library item summary, nil for free text blocks.
	ItemID           *uuid.UUID `db:"item_id" json:"-"`
	ItemKind         *string    `db:"item_kind" json:"-"`
	ItemTitle        *string    `db:"item_title" json:"-"`
	ItemLocationName *string    `db:"item_location_name" json:"-"`
	ItemCategory     *string    `db:"item_category" json:"-"`
}

type BlockItemSummary struct {
	ID           uuid.UUID `json:"id"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	LocationName *string   `json:"location_name"`
	Category     *string   `json:"category"`
}

// BlockView is the API shape of a block, including the item summary so a
// timeline renders from one request.
type BlockView struct {
	ID            uuid.UUID         `json:"id"`
	TripID        uuid.UUID         `json:"trip_id"`
	LibraryItemID *uuid.UUID        `json:"library_item_id"`
	LibraryItem   *BlockItemSummary `json:"library_item"`
	Title         *string           `json:"title"`
	Note          string            `json:"note"`
	Hue           *int              `json:"hue"`
	StartAt       time.Time         `json:"start_at"`
	EndAt         time.Time         `json:"end_at"`
	CreatedBy     *uuid.UUID        `json:"created_by"`
	UpdatedBy     *uuid.UUID        `json:"updated_by"`
	Version       int               `json:"version"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

func (b ScheduleBlock) View() BlockView {
	view := BlockView{
		ID:            b.ID,
		TripID:        b.TripID,
		LibraryItemID: b.LibraryItemID,
		Title:         b.Title,
		Note:          b.Note,
		Hue:           b.Hue,
		StartAt:       b.StartAt,
		EndAt:         b.EndAt,
		CreatedBy:     b.CreatedBy,
		UpdatedBy:     b.UpdatedBy,
		Version:       b.Version,
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
	}

	if b.ItemID != nil && b.ItemKind != nil && b.ItemTitle != nil {
		view.LibraryItem = &BlockItemSummary{
			ID:           *b.ItemID,
			Kind:         *b.ItemKind,
			Title:        *b.ItemTitle,
			LocationName: b.ItemLocationName,
			Category:     b.ItemCategory,
		}
	}

	return view
}
