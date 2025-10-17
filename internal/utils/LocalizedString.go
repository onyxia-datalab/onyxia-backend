package utils

import "fmt"

type LocalizedStringType string

const (
	LocalizedStringTypePlain LocalizedStringType = "plain"
	LocalizedStringTypeMulti LocalizedStringType = "multi"
)

type MultiLangString map[string]string

type LocalizedString struct {
	typ   LocalizedStringType
	plain string
	multi MultiLangString
}

func NewLocalizedString(value interface{}) (LocalizedString, error) {
	switch v := value.(type) {
	case string:
		return LocalizedString{
			typ:   LocalizedStringTypePlain,
			plain: v,
		}, nil

	case MultiLangString:
		if len(v) == 0 {
			return LocalizedString{}, fmt.Errorf("multi-lang string cannot be empty")
		}
		return LocalizedString{
			typ:   LocalizedStringTypeMulti,
			multi: v,
		}, nil

	case nil:
		return LocalizedString{}, fmt.Errorf("nil value provided")

	default:
		return LocalizedString{}, fmt.Errorf("unsupported type %T for LocalizedString", v)
	}
}

func (s LocalizedString) IsPlain() bool {
	return s.typ == LocalizedStringTypePlain
}

func (s LocalizedString) IsMulti() bool {
	return s.typ == LocalizedStringTypeMulti
}

func (s LocalizedString) GetPlain() (string, bool) {
	if s.IsPlain() {
		return s.plain, true
	}
	return "", false
}

func (s LocalizedString) GetMulti() (MultiLangString, bool) {
	if s.IsMulti() {
		return s.multi, true
	}
	return nil, false
}

func (s LocalizedString) Type() LocalizedStringType {
	return s.typ
}
