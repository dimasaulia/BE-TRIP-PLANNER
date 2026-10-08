package entities

import (
	"time"

	"github.com/google/uuid"
)

type Attachment struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	TripID       uuid.UUID  `db:"trip_id" json:"trip_id"`
	UploadedBy   *uuid.UUID `db:"uploaded_by" json:"uploaded_by"`
	StoredName   string     `db:"stored_name" json:"stored_name"`
	OriginalName string     `db:"original_name" json:"original_name"`
	Mime         string     `db:"mime" json:"mime"`
	SizeBytes    int64      `db:"size_bytes" json:"size_bytes"`
	Width        int        `db:"width" json:"width"`
	Height       int        `db:"height" json:"height"`
	SHA256       string     `db:"sha256" json:"sha256"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
}
