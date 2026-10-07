package dto

import "time"

type Status struct {
	Status    string     `json:"status"`
	StartedAt *time.Time `json:"started_at,omitempty"`
}
