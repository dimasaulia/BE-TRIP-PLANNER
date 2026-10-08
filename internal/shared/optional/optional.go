// Package optional distinguishes an absent JSON field from an explicit null,
// which PATCH bodies need for nullable columns.
package optional

import "encoding/json"

type Value[T any] struct {
	Set   bool
	Null  bool
	Value T
}

func (o *Value[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

// Ptr returns nil when the field was absent or null.
func (o Value[T]) Ptr() *T {
	if !o.Set || o.Null {
		return nil
	}
	value := o.Value
	return &value
}
