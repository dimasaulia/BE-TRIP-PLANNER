package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateInviteRequest struct {
	Role          string `json:"role"`
	ExpiresInDays *int   `json:"expires_in_days"`
	MaxUses       *int   `json:"max_uses"`
}

// CreateInviteResponse is the only time the token and its link are shown.
type CreateInviteResponse struct {
	ID        uuid.UUID `json:"id"`
	TripID    uuid.UUID `json:"trip_id"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	MaxUses   *int      `json:"max_uses"`
	UseCount  int       `json:"use_count"`
	Token     string    `json:"token"`
	URL       string    `json:"url"`
}

type InviteItem struct {
	ID            uuid.UUID `json:"id"`
	Role          string    `json:"role"`
	CreatedBy     uuid.UUID `json:"created_by"`
	CreatedByName string    `json:"created_by_name"`
	ExpiresAt     time.Time `json:"expires_at"`
	MaxUses       *int      `json:"max_uses"`
	UseCount      int       `json:"use_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type PreviewResponse struct {
	TripName  string    `json:"trip_name"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AcceptResponse struct {
	TripID uuid.UUID `json:"trip_id"`
	Role   string    `json:"role"`
	// Joined is false when the caller already was a member (idempotent accept).
	Joined bool `json:"joined"`
}
