package domain

import "errors"

var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrForbidden     = errors.New("forbidden")      // business rule denial
	ErrAlreadyExists = errors.New("already exists") // idempotency/conflict
	ErrNotFound      = errors.New("not found")
	ErrNotSupported  = errors.New("operation not supported") // e.g. chart doesn't support global.suspend
)
