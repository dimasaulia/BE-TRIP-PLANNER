package entities

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `db:"id" json:"id"`
	GoogleSub string    `db:"google_sub" json:"-"`
	Email     string    `db:"email" json:"email"`
	Name      string    `db:"name" json:"name"`
	AvatarURL *string   `db:"avatar_url" json:"avatar_url"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
