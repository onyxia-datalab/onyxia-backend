package domain

import "github.com/onyxia-datalab/onyxia-backend/internal/apperror"

// These re-export the shared REST-error vocabulary (internal/apperror) under
// this API's own domain package, so controller/usecase code keeps using its
// familiar domain.ErrXxx names. They are the same underlying error values,
// so errors.Is works identically through either name.
var (
	ErrForbidden = apperror.ErrForbidden
)
