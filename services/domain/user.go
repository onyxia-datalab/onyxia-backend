package domain

// type User struct {
// 	username   string
// 	groups     []string
// 	roles      []string
// 	attributes map[string]string
// }

// func NewUser(username string, groups, roles []string, attrs map[string]string) User {
// 	gs := append([]string(nil), groups...)
// 	rs := append([]string(nil), roles...)
// 	var as map[string]string
// 	if attrs != nil {
// 		as = make(map[string]string, len(attrs))
// 		for k, v := range attrs {
// 			as[k] = v
// 		}
// 	}
// 	return User{username: username, groups: gs, roles: rs, attributes: as}
// }

// // Read-only accessors (domain stays in control of invariants).
// func (u User) Username() string { return u.username }
// func (u User) Groups() []string { return append([]string(nil), u.groups...) }
// func (u User) Roles() []string  { return append([]string(nil), u.roles...) }
// func (u User) Attributes() map[string]string {
// 	if u.attributes == nil {
// 		return nil
// 	}
// 	cp := make(map[string]string, len(u.attributes))
// 	for k, v := range u.attributes {
// 		cp[k] = v
// 	}
// 	return cp
// }
