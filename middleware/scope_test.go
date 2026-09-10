package middleware

import (
	"testing"

	"gusseynov/GO-Registry/config"
)

// setupRoles готовит конфиг ролей: без него IsSuperAdmin/IsBossPost работают
// с пустыми списками и любой Scope выходит пустым.
func setupRoles(t *testing.T) {
	t.Helper()

	old := config.Cfg
	t.Cleanup(func() { config.Cfg = old })

	config.Cfg = &config.Config{
		ApproveAdmins: []string{"Гусейнов Шамиль Аладдинович"},
		Boss:          []string{"Директор", "Руководитель", "Управляющий директор"},
	}
}

func TestScope(t *testing.T) {
	setupRoles(t)

	cases := []struct {
		name     string
		page     BasePageContext
		wantAll  bool
		wantDeps []string
	}{
		{
			name:    "супер-администратор видит всё",
			page:    BasePageContext{FIO: "Гусейнов Шамиль Аладдинович", DepName: "ДИТ"},
			wantAll: true,
		},
		{
			name: "управляющий директор — только подчинённые департаменты",
			// Своего департамента в списке нет и не должно быть: GO-SSO
			// исключает собственный DN, а сотрудников «при себе» у
			// управляющего директора нет (решение пользователя 10.09.2026).
			page: BasePageContext{
				FIO: "Иванов И И", Post: "Управляющий директор", DepName: "Аппарат",
				SubordinateOU: []string{"Департамент А", "Департамент Б"},
			},
			wantDeps: []string{"Департамент А", "Департамент Б"},
		},
		{
			name:     "директор департамента — свой департамент",
			page:     BasePageContext{FIO: "Петров П П", Post: "Директор департамента", DepName: "Департамент А"},
			wantDeps: []string{"Департамент А"},
		},
		{
			name: "рядовой сотрудник — никого",
			page: BasePageContext{FIO: "Сидоров С С", Post: "Главный специалист", DepName: "Департамент А"},
		},
		{
			name: "аноним — никого",
			page: BasePageContext{IsAnonymous: true, FIO: "Гусейнов Шамиль Аладдинович"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.page.Scope()

			if got.All != c.wantAll {
				t.Fatalf("All = %v, ожидалось %v", got.All, c.wantAll)
			}
			if len(got.Departments) != len(c.wantDeps) {
				t.Fatalf("Departments = %v, ожидалось %v", got.Departments, c.wantDeps)
			}
			for i, d := range c.wantDeps {
				if got.Departments[i] != d {
					t.Errorf("Departments[%d] = %q, ожидалось %q", i, got.Departments[i], d)
				}
			}
		})
	}
}

// TestScopeAllows — то, ради чего Scope и заведён: пустит ли он к чужой заявке.
func TestScopeAllows(t *testing.T) {
	setupRoles(t)

	admin := BasePageContext{FIO: "Гусейнов Шамиль Аладдинович"}
	bigBoss := BasePageContext{
		FIO: "Иванов И И", Post: "Управляющий директор", DepName: "Аппарат",
		SubordinateOU: []string{"Департамент А", "Департамент Б"},
	}
	smallBoss := BasePageContext{FIO: "Петров П П", Post: "Директор департамента", DepName: "Департамент А"}
	staff := BasePageContext{FIO: "Сидоров С С", Post: "Главный специалист", DepName: "Департамент А"}

	cases := []struct {
		name string
		page BasePageContext
		dep  string
		want bool
	}{
		{"админ — чужой департамент", admin, "Департамент В", true},
		{"админ — неизвестный департамент", admin, "чего-то нет в справочнике", true},

		{"управляющий — свой подчинённый", bigBoss, "Департамент А", true},
		{"управляющий — второй подчинённый", bigBoss, "Департамент Б", true},
		{"управляющий — чужой департамент", bigBoss, "Департамент В", false},
		// Собственный департамент управляющего в subordinate_ou не приходит,
		// значит и заявки оттуда он не решает — так устроено намеренно.
		{"управляющий — свой собственный", bigBoss, "Аппарат", false},

		{"директор — свой департамент", smallBoss, "Департамент А", true},
		{"директор — соседний департамент", smallBoss, "Департамент Б", false},
		// Регистр и пробелы: dep_name приходит из LDAP и из колонки register,
		// совпадать байт в байт они не обязаны.
		{"директор — свой, другой регистр", smallBoss, "  департамент а ", true},

		{"сотрудник — свой департамент", staff, "Департамент А", false},
		{"пустой департамент никому", admin, "", true},
		{"пустой департамент руководителю", smallBoss, "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.page.Scope().Allows(c.dep); got != c.want {
				t.Errorf("Allows(%q) = %v, ожидалось %v", c.dep, got, c.want)
			}
		})
	}
}
