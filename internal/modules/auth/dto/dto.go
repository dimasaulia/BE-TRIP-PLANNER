package dto

import "github.com/google/uuid"

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url"`
}

// CalendarStatus tells the frontend whether to offer "connect" or "reconnect".
type CalendarStatus struct {
	Connected   bool    `json:"connected"`
	Status      string  `json:"status"` // not_connected | active | revoked
	GoogleEmail *string `json:"google_email"`
}

type MeResponse struct {
	User     UserResponse   `json:"user"`
	Calendar CalendarStatus `json:"calendar"`
}

type DevLoginRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}
