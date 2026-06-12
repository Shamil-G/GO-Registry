package logging

import (
	"context"
	"log/slog"
)

type LevelFilter struct {
	minLevel slog.Level
	next     slog.Handler
}

func NewLevelFilter(min slog.Level, next slog.Handler) slog.Handler {
	return &LevelFilter{minLevel: min, next: next}
}

func (h *LevelFilter) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.minLevel && h.next.Enabled(ctx, level)
}

func (h *LevelFilter) Handle(ctx context.Context, r slog.Record) error {
	if r.Level < h.minLevel {
		return nil
	}
	return h.next.Handle(ctx, r)
}

func (h *LevelFilter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LevelFilter{minLevel: h.minLevel, next: h.next.WithAttrs(attrs)}
}

func (h *LevelFilter) WithGroup(name string) slog.Handler {
	return &LevelFilter{minLevel: h.minLevel, next: h.next.WithGroup(name)}
}
