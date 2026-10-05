package domain

import "github.com/onyxia-datalab/onyxia-backend/internal/usercontext"

// OnboardingRequest asks to onboard User's personal namespace, or the
// namespace of Group when it is set.
type OnboardingRequest struct {
	User  usercontext.User
	Group *string
}
