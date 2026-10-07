package dto

type ListResponse[T any] struct {
	Items []T `json:"items"`
}
