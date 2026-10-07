package entities

import "time"

type ReleaseNote struct {
	ID          int64      `db:"id" json:"id"`
	Version     string     `db:"version" json:"version"`
	Category    int16      `db:"category" json:"category"`
	Title       string     `db:"title" json:"title"`
	ParentID    *int64     `db:"parent_id" json:"parent_id,omitempty"`
	Notes       string     `db:"notes" json:"notes"`
	ReleaseDate time.Time  `db:"release_date" json:"release_date"`
	Visibility  int16      `db:"visibility" json:"visibility"`
	IsActive    *int16     `db:"is_active" json:"is_active,omitempty"`
	CreatedAt   *time.Time `db:"created_at" json:"created_at,omitempty"`
	UpdatedAt   *time.Time `db:"updated_at" json:"updated_at,omitempty"`
	DeletedAt   *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
	CreatedBy   *int64     `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy   *int64     `db:"updated_by" json:"updated_by,omitempty"`
	DeletedBy   *int64     `db:"deleted_by" json:"deleted_by,omitempty"`
}
