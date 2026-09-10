// service/register.go
//
// Чтение заявок на отсутствие. Пока здесь только то, что нужно проверкам прав;
// вынос остальных SELECT-ов из web/*.go — пункт 2.4 плана.
package service

import (
	"context"
	"strings"

	"gusseynov/GO-Registry/storage"
)

// RegisterDep возвращает департамент заявки и ФИО сотрудника по её id.
//
// Нужна для серверной проверки в ApproveTimeOffPost/RefuseTimeOffPost: id
// приходит скрытым полем формы, поэтому «чья это заявка» спрашиваем у БД, а не
// у браузера. Ровно тот же приём, что в service.MessageOwner.
//
// found=false — заявки с таким id нет (обычно устаревшая вкладка: заявку уже
// удалили или согласовали и она ушла из списка).
func RegisterDep(ctx context.Context, idReg int) (depName, employee string, found bool, err error) {
	// MAX(...) + CHR(1): агрегат всегда возвращает строку, поэтому sql.ErrNoRows
	// здесь не бывает и штатное «заявки уже нет» не пишет ERROR в лог.
	// Подробнее — в комментарии к MessageOwner.
	const query = `
		SELECT COALESCE(MAX(dep_name), ' ') || CHR(1) || COALESCE(MAX(employee), ' ')
		  FROM register
		 WHERE id = :1`

	var row string
	if err = storage.DBSelectOne(ctx, "register_dep", &row, query, idReg); err != nil {
		return "", "", false, err
	}

	depName, employee, _ = strings.Cut(row, "\x01")
	depName = strings.TrimSpace(depName)
	employee = strings.TrimSpace(employee)

	// Департамент — обязательная колонка заявки, поэтому его пустота означает
	// именно отсутствие строки, а не заявку без департамента.
	return depName, employee, depName != "", nil
}
