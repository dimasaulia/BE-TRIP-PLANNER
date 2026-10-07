package dto

type CreateReleaseNoteRequest struct {
	Version     string `json:"version"`
	Category    int16  `json:"category"`
	Title       string `json:"title"`
	ParentID    *int64 `json:"parent_id"`
	Notes       string `json:"notes"`
	ReleaseDate string `json:"release_date"`
	Visibility  int16  `json:"visibility"`
	IsActive    *int16 `json:"is_active"`
	CreatedBy   *int64 `json:"created_by"`
}

type UpdateReleaseNoteRequest struct {
	Version     *string `json:"version"`
	Category    *int16  `json:"category"`
	Title       *string `json:"title"`
	ParentID    *int64  `json:"parent_id"`
	Notes       *string `json:"notes"`
	ReleaseDate *string `json:"release_date"`
	Visibility  *int16  `json:"visibility"`
	IsActive    *int16  `json:"is_active"`
	UpdatedBy   *int64  `json:"updated_by"`
}
