// config/access.go
package config

import (
	"log/slog"
	"strings"
)

// IsBossPost проверяет, относится ли должность сотрудника к руководящему составу
func IsBossPost(userPost string) bool {
	if userPost == "" {
		slog.Warn("IsBossPost", "Передана пустая должность", userPost)
		return false
	}
	for _, post := range Cfg.Boss {
		// Содержит ли должность пользователя (userPost) в себе одну из строк из списка руководящих должностей (post)
		if strings.Contains(strings.ToLower(userPost), strings.ToLower(strings.TrimSpace(post))) {
			return true
		}
	}
	return false
}

// IsSuperAdmin проверяет, является ли пользователь супер-администратором по ФИО
func IsSuperAdmin(loginName string) bool {
	if loginName == "" {
		slog.Warn("IsSuperAdmin", "Передан пустой loginName", loginName)
		return false
	}
	for _, adminName := range Cfg.ApproveAdmins {
		if strings.TrimSpace(strings.ToLower(loginName)) == strings.TrimSpace(strings.ToLower(adminName)) {
			return true
		}
	}
	return false
}

// IsHR проверяет, относится ли должность сотрудника к HR-отделу
func IsHR(userDep string) bool {
	for _, hrDep := range Cfg.HRList {
		if strings.EqualFold(userDep, hrDep) {
			return true
		}
	}
	return false
}

// IsSecurity — относится ли департамент сотрудника к службе безопасности.
//
// СБ ведёт свой список отсутствий (/secure-time-off) и отслеживает приход на
// работу, поэтому раздел закрыт именно по департаменту, а не по должности:
// в СБ этим занимается не только руководитель.
//
// Пустой SECURITY_DEPARTMENT означает «никто» — раздел остаётся у
// супер-администраторов. Это осознанный дефолт: лучше лишний раз позвать
// администратора, чем открыть чужой список всем.
func IsSecurity(userDep string) bool {
	for _, secDep := range Cfg.SecurityList {
		if strings.EqualFold(strings.TrimSpace(userDep), strings.TrimSpace(secDep)) {
			return true
		}
	}
	return false
}
