package middleware

import (
	"context"
)

// Уникальные типы ключей контекста для защиты от коллизий имен
type userContextKey struct{}

var UserKey = userContextKey{}

// BasePageContext содержит информацию о сессии и все готовые флаги для шаблона base.html
type BasePageContext struct {
	Lang          string // Язык с дефолтом
	Theme         string // Тема с дефолтом
	FIO           string // ФИО (user.FIO)
	LoginName     string // Имя введенное при регисрации
	RfbnID        string // Код региона
	LegacyName    string //
	DepName       string // Департамент (user.DepName)
	IP            string
	Post          string // Должность
	IsAnonymous   bool   // Флаг для верстки
	IsBoss        bool   // Флаг руководителя (считается один раз здесь)
	Time          string
	SubordinateOU []string
}

func GetOrCreatePageCtx(ctx context.Context) *BasePageContext {
	if v, ok := ctx.Value(UserKey).(*BasePageContext); ok && v != nil {
		return v
	}
	return &BasePageContext{}
}

func SavePageCtx(ctx context.Context, page *BasePageContext) context.Context {
	return context.WithValue(ctx, UserKey, page)
}
