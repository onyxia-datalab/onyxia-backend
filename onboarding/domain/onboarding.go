package domain

type OnboardingRequest struct {
	Group     *string // Use pointer to indicate optional value
	UserName  string
	UserRoles []string
}
