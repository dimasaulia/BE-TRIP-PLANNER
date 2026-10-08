package validation

import "github.com/open-suite/boilerplate-golang/internal/shared/apperror"

// Validator collects field level problems so a request reports all of them at once.
type Validator struct {
	fields map[string]string
}

func New() *Validator {
	return &Validator{fields: map[string]string{}}
}

// Check records message for field when ok is false. The first message per field wins.
func (v *Validator) Check(ok bool, field string, message string) {
	if ok {
		return
	}
	if _, exists := v.fields[field]; !exists {
		v.fields[field] = message
	}
}

func (v *Validator) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return apperror.BadRequest("validation_error").With("fields", v.fields)
}

// ErrWith is a shortcut for a single failing field.
func (v *Validator) ErrWith(field string, message string) error {
	v.Check(false, field, message)
	return v.Err()
}
