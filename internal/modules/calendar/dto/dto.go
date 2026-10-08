package dto

import "time"

type SyncStatusResponse struct {
	Enabled      bool       `json:"enabled"`
	CalendarID   string     `json:"google_calendar_id,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at"`
	LastError    *string    `json:"last_error"`
	PendingJobs  int        `json:"pending_jobs"`
}
