package app

import "errors"

var (
	ErrValidation      = errors.New("validation")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("idempotency conflict")
	ErrBusiness        = errors.New("business rule")
	ErrInactiveSpace   = errors.New("space inactive")
	ErrOverlap         = errors.New("confirmed overlap")
	ErrInPast          = errors.New("start in the past")
	ErrTooLong         = errors.New("duration too long")
	ErrTransition      = errors.New("invalid status transition")
	ErrJWKSUnavailable = errors.New("jwks unavailable")
)
