package domain

import "errors"

var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrForbidden     = errors.New("forbidden")      // business rule denial
	ErrAlreadyExists = errors.New("already exists") // idempotency/conflict
)
