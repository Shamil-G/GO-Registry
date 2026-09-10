// storage/error_label.go
//
// Нормализация ошибки Oracle для метки Prometheus (пункт 1.6 плана, 10.09.2026).
//
// Было: DBSPErrors.WithLabelValues(procName, err.Error()) — полный текст
// ошибки как значение метки. Две беды сразу:
//
//  1. Кардинальность. Текст ORA-ошибки содержит номера строк, имена объектов, а
//     иногда и значения из запроса — то есть почти каждая ошибка уникальна.
//     Каждая уникальная метка создаёт в Prometheus отдельную временную серию,
//     и они не исчезают до перезапуска. Так метрика ошибок постепенно кладёт
//     тот самый мониторинг, ради которого её заводили.
//  2. Утечка. Метки видны на /metrics, то есть тексты ORA-ошибок читал любой,
//     кто открыл эту страницу.
//
// Стало: код вида ORA-01722. Их конечное число, для алертов и графиков этого
// достаточно, а полный текст ошибки как был, так и остаётся в логе рядом.
package storage

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

// oraCode — "ORA-01722: invalid number" → "ORA-01722".
// Регистр разный у разных драйверов, поэтому (?i).
var oraCode = regexp.MustCompile(`(?i)ORA-\d{5}`)

// errorLabel — короткая метка ошибки для Prometheus.
func errorLabel(err error) string {
	if err == nil {
		return ""
	}

	if code := oraCode.FindString(err.Error()); code != "" {
		return strings.ToUpper(code)
	}

	// Не-ORA ошибки: сеть, отменённый запрос, мёртвое соединение. Их немного,
	// и различать их полезно — но тоже фиксированным набором значений.
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "other"
	}
}
