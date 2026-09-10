// middleware/scope.go
//
// Область видимости руководителя — один ответ на вопрос «чьи заявки этот
// человек вправе смотреть и согласовывать» (10.09.2026, пункт 0.4 плана).
//
// Правило, подтверждённое пользователем 10.09.2026:
//
//   - супер-администратор (ApproveAdmins) — все департаменты;
//   - управляющий директор — РОВНО те департаменты, что пришли из SSO в
//     subordinate_ou. Своего департамента у него в списке нет и не должно
//     быть: в GO-SSO (storage/ldap/ldap_deps.go) собственный DN из списка
//     исключается намеренно, и сотрудников «при себе» у управляющего
//     директора нет — все они в подчинённых департаментах;
//   - директор департамента (должность из списка Boss) — свой департамент;
//   - все остальные — никого.
//
// Логика взята из ListToApproveGet, где она работала и была проверена; здесь
// она вынесена ровно затем, чтобы согласование и отказ спрашивали ТО ЖЕ САМОЕ,
// что и список. До этой правки список фильтровался, а approve/refuse брали id
// из формы и не сверяли ничего — руководитель департамента А мог согласовать
// заявку департамента Б, поправив скрытое поле в браузере.
package middleware

import (
	"strings"

	"gusseynov/GO-Registry/config"
)

// Scope — что человеку видно.
//
// All=true значит «все департаменты», и тогда Departments не заполняется:
// у администратора нет списка, у него нет ограничения.
type Scope struct {
	All         bool
	Departments []string
}

// Scope считает область видимости по данным сессии.
func (p *BasePageContext) Scope() Scope {
	if p == nil || p.IsAnonymous {
		return Scope{}
	}

	if config.IsSuperAdmin(p.FIO) {
		return Scope{All: true}
	}

	// Управляющий директор: только подчинённые департаменты из SSO.
	if len(p.SubordinateOU) > 0 {
		return Scope{Departments: p.SubordinateOU}
	}

	// Директор департамента: свой департамент.
	if config.IsBossPost(p.Post) && strings.TrimSpace(p.DepName) != "" {
		return Scope{Departments: []string{p.DepName}}
	}

	return Scope{}
}

// Allows — попадает ли департамент в область видимости.
//
// Сравнение регистронезависимое и без краевых пробелов: dep_name приходит из
// двух разных мест — из LDAP через SSO и из колонки register.dep_name, куда он
// когда-то был записан, — и совпадать байт в байт они не обязаны.
func (s Scope) Allows(depName string) bool {
	if s.All {
		return true
	}

	depName = strings.TrimSpace(depName)
	if depName == "" {
		return false
	}

	for _, d := range s.Departments {
		if strings.EqualFold(strings.TrimSpace(d), depName) {
			return true
		}
	}
	return false
}

// Empty — область пуста, то есть человек не руководитель ни для кого.
func (s Scope) Empty() bool {
	return !s.All && len(s.Departments) == 0
}
