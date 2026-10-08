package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/shared/optional"
)

type CreateTripRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Timezone    string `json:"timezone"`
	// BannerURL is an http(s) image URL; upload through the uploads API for an app-hosted one.
	BannerURL *string `json:"banner_url"`
}

type UpdateTripRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	StartDate   *string `json:"start_date"`
	EndDate     *string `json:"end_date"`
	Timezone    *string `json:"timezone"`
	// BannerURL: omit to keep, null to remove, a URL to set.
	BannerURL optional.Value[string] `json:"banner_url"`
}

type UpdateMemberRequest struct {
	Role string `json:"role"`
}

type TripResponse struct {
	entities.Trip
	MyRole string `json:"my_role"`
}

type TripListItem struct {
	entities.Trip
	MyRole      string `json:"my_role"`
	MemberCount int    `json:"member_count"`
}

type CalendarSyncStatus struct {
	Enabled      bool       `json:"enabled"`
	LastSyncedAt *time.Time `json:"last_synced_at"`
	LastError    *string    `json:"last_error"`
	PendingJobs  int        `json:"pending_jobs"`
}

type TripDetailResponse struct {
	entities.Trip
	MyRole       string                `json:"my_role"`
	Members      []entities.TripMember `json:"members"`
	CalendarSync CalendarSyncStatus    `json:"calendar_sync"`
}

type MemberChange struct {
	Action string               `json:"action"` // added | role_changed | removed
	UserID uuid.UUID            `json:"user_id"`
	Member *entities.TripMember `json:"member,omitempty"`
}
