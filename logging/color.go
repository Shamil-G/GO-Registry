// logging/color.go

package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

type ColorHandler struct {
	h slog.Handler
}

func NewColorHandler(opts *slog.HandlerOptions) *ColorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	// ВАЖНО: заставляем базовый TextHandler УДАЛЯТЬ стандартный level из строки,
	// потому что мы его выведем вручную цветом через fmt.Fprintf
	opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.LevelKey {
			return slog.Attr{} // возвращаем пустой атрибут, он стирается из лога
		}
		return a
	}

	return &ColorHandler{
		h: slog.NewTextHandler(os.Stdout, opts),
	}
}

func (c *ColorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return c.h.Enabled(ctx, level)
}

func (c *ColorHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	color := ""

	switch r.Level {
	case slog.LevelDebug:
		color = "\033[36m" // бирюзовый
	case slog.LevelInfo:
		color = "\033[32m" // зеленый
	case slog.LevelWarn:
		color = "\033[33m" // желтый
	case slog.LevelError:
		color = "\033[31m" // красный
	case LevelStart:
		color = "\033[38;5;207m" // сиреневый (pink/magenta)
		level = "START/STOP"
	}

	reset := "\033[0m"

	// Пишем цветной префикс прямо в консоль, как в GO-SSO
	fmt.Fprintf(os.Stdout, "%s[%s]%s ", color, level, reset)

	// Вызываем базовый обработчик (он выведет время, msg и остальные атрибуты БЕЗ лишнего level)
	return c.h.Handle(ctx, r)
}

func (c *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColorHandler{h: c.h.WithAttrs(attrs)}
}

func (c *ColorHandler) WithGroup(name string) slog.Handler {
	return &ColorHandler{h: c.h.WithGroup(name)}
}

func Start(msg string, args ...any) {
	slog.Log(context.Background(), LevelStart, msg, args...)
}
