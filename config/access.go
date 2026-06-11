// config/access.go
package config

import (
	"strings"
)

// IsBossPost проверяет, относится ли должность сотрудника к руководящему составу
func IsBossPost(userPost string) bool {
	if userPost == "" {
		return false
	}
	for _, post := range Cfg.Boss {
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
	for _, adminName := range Cfg.ApproveAdmins {
		if strings.TrimSpace(strings.ToLower(loginName)) == strings.TrimSpace(strings.ToLower(adminName)) {
			return true
		}
	}
	return false
}
