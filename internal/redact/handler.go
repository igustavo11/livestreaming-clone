package redact

import (
	"context"
	"log/slog"
	"strings"
)

type redactingHandler struct {
	base    slog.Handler
	secrets []string
}

func NewRedactingHandler(base slog.Handler, secrets []string) slog.Handler {
	// filter empty
	var filtered []string
	for _, s := range secrets {
		if s != "" {
			filtered = append(filtered, s)
		}
	}
	return &redactingHandler{base: base, secrets: filtered}
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	// redact message
	r.Message = h.redactString(r.Message)
	// redact attrs
	// we need to create a new record with redacted attrs
	// slog.Record is a struct, we can modify its attrs via r.Attrs and then AddAttrs
	// Simpler: create a new record and add redacted attrs
	newR := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	// copy attrs with redaction
	r.Attrs(func(a slog.Attr) bool {
		newR.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.base.Handle(ctx, newR)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		redacted = append(redacted, h.redactAttr(a))
	}
	return &redactingHandler{base: h.base.WithAttrs(redacted), secrets: h.secrets}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{base: h.base.WithGroup(h.redactString(name)), secrets: h.secrets}
}

func (h *redactingHandler) redactAttr(a slog.Attr) slog.Attr {
	// redact key and value
	a.Key = h.redactString(a.Key)
	// redact value based on kind
	switch a.Value.Kind() {
	case slog.KindString:
		a.Value = slog.StringValue(h.redactString(a.Value.String()))
	case slog.KindGroup:
		// need to redact group attrs recursively
		grp := a.Value.Group()
		redacted := make([]slog.Attr, 0, len(grp))
		for _, ga := range grp {
			redacted = append(redacted, h.redactAttr(ga))
		}
		a.Value = slog.GroupValue(redacted...)
	default:
		// for other kinds, try to redact string representation if it contains secret
		// we check the string value and if it contains secret, replace
		s := a.Value.String()
		redacted := h.redactString(s)
		if redacted != s {
			// if redacted, replace with string
			a.Value = slog.StringValue(redacted)
		}
	}
	return a
}

func (h *redactingHandler) redactString(s string) string {
	if s == "" || len(h.secrets) == 0 {
		return s
	}
	for _, sec := range h.secrets {
		if sec == "" {
			continue
		}
		if strings.Contains(s, sec) {
			s = strings.ReplaceAll(s, sec, "[REDACTED]")
		}
	}
	return s
}
