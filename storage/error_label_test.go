package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestErrorLabel(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"ORA с текстом", errors.New("ORA-01722: invalid number"), "ORA-01722"},
		{"ORA в нижнем регистре", errors.New("ora-00001: unique constraint violated"), "ORA-00001"},
		{"ORA внутри длинного текста", errors.New("oracle: failed to execute: ORA-06512: at \"REGISTRY.REG\", line 214"), "ORA-06512"},
		{"обёрнутая ошибка", fmt.Errorf("вызов reg.del_message: %w", errors.New("ORA-00942: table or view does not exist")), "ORA-00942"},
		{"отменённый запрос", context.Canceled, "canceled"},
		{"таймаут", context.DeadlineExceeded, "timeout"},
		{"сетевая ошибка", errors.New("dial tcp 192.168.20.60:1521: connect: connection refused"), "other"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := errorLabel(c.err); got != c.want {
				t.Errorf("errorLabel(%v) = %q, ожидалось %q", c.err, got, c.want)
			}
		})
	}
}

// TestErrorLabelBounded — главное свойство метки: у сотни разных ORA-ошибок
// одного вида должно получиться ОДНО значение, иначе Prometheus обрастает
// временными сериями и со временем ложится.
func TestErrorLabelBounded(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		err := fmt.Errorf("ORA-06512: at \"REGISTRY.REG\", line %d", i)
		seen[errorLabel(err)] = true
	}
	if len(seen) != 1 {
		t.Errorf("сто ошибок одного вида дали %d разных меток: %v", len(seen), seen)
	}
}
