package validation

import "github.com/go-playground/validator/v10"

// New creates the shared request/application input validator.
func New() *validator.Validate {
	return validator.New()
}
