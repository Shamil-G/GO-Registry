// config/access.go
package config

import (
	"strings"
)

// Tester — параметры тестирования (перенесено из FastAPI)
var Tester = map[string]any{
	"login_name": "Гусейнов",
	"top_level":  2,
	"top_view":   0,
}

// Boss — список ключевых названий должностей руководителей любого уровня
var Boss = []string{
	"Начальник отдела",
	"Заведующий сектором",
	"Директор департамента",
	"Начальник управления",
}

// ApproveAdmins — список ФИО супер-администраторов с абсолютными правами
var ApproveAdmins = []string{
	"Гусейнов Шамиль Аладдинович",
	"Алибаева Мадина Жасулановна",
	"Маликов Айдар Амангельдыевич",
	"Сарузенов Ғабит Маратұлы",
}

// IsBossPost проверяет, относится ли должность сотрудника к руководящему составу
func IsBossPost(userPost string) bool {
	if userPost == "" {
		return false
	}
	for _, post := range Boss {
		if strings.Contains(strings.ToLower(userPost), strings.ToLower(strings.TrimSpace(post))) {
			return true
		}
	}
	return false
}

// IsSuperAdmin проверяет, является ли пользователь супер-администратором по ФИО
func IsSuperAdmin(loginName string) bool {
	if loginName == "" {
		return false
	}
	for _, adminName := range ApproveAdmins {
		if strings.TrimSpace(strings.ToLower(loginName)) == strings.TrimSpace(strings.ToLower(adminName)) {
			return true
		}
	}
	return false
}
