// service/i18n.go

package i18n

func Get(lang, key string) string {
	var dict map[string]string

	switch lang {
	case "kz":
		dict = KZ
	default:
		dict = RU
	}

	if v, ok := dict[key]; ok {
		return v
	}
	return key
}
