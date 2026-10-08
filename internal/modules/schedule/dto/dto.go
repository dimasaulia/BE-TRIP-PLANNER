package dto

import (
	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/shared/optional"
	"github.com/open-suite/boilerplate-golang/internal/shared/timeutil"
)

type CreateBlockRequest struct {
	LibraryItemID *uuid.UUID `json:"library_item_id"`
	Title         *string    `json:"title"`
	Note          string     `json:"note"`
	// Hue (0–359) colours a free-text note; null keeps the neutral note.
	Hue     *int          `json:"hue"`
	StartAt *timeutil.UTC `json:"start_at"`
	EndAt   *timeutil.UTC `json:"end_at"`
}

// UpdateBlockRequest covers move, resize and change of content in one PATCH.
type UpdateBlockRequest struct {
	Version       *int                      `json:"version"`
	LibraryItemID optional.Value[uuid.UUID] `json:"library_item_id"`
	Title         optional.Value[string]    `json:"title"`
	Note          optional.Value[string]    `json:"note"`
	Hue           optional.Value[int]       `json:"hue"`
	StartAt       *timeutil.UTC             `json:"start_at"`
	EndAt         *timeutil.UTC             `json:"end_at"`
}

type ListResponse struct {
	Items []entities.BlockView `json:"items"`
}
