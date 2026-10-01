package domain

import "github.com/onyxia-datalab/onyxia-backend/internal/apperror"

// These re-export the shared REST-error vocabulary (internal/apperror) under
// this API's own domain package, so usecase/controller code keeps using its
// familiar domain.ErrXxx names. They are the same underlying error values,
// so errors.Is works identically through either name.
var (
	ErrInvalidInput  = apperror.ErrInvalidInput
	ErrForbidden     = apperror.ErrForbidden     // business rule denial
	ErrAlreadyExists = apperror.ErrAlreadyExists // idempotency/conflict
	ErrNotFound      = apperror.ErrNotFound
	ErrNotSupported  = apperror.ErrNotSupported // e.g. chart doesn't support global.suspend
)
