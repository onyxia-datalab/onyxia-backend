package usercontext

func NewUserContext() (Reader, Writer) {
	uc := userContext{}
	return uc, uc
}
