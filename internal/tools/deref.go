package tools

// Deref safely returns the value pointed to by p,
// or the zero value if p is nil.
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
