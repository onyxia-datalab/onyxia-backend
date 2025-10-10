package utils

type LocalizedString map[string]string

func (l LocalizedString) Get(lang string) string {
	if v, ok := l[lang]; ok {
		return v
	}
	if v, ok := l["en"]; ok {
		return v
	}
	for _, v := range l {
		return v
	}
	return ""
}
