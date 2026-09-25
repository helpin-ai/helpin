package observability

import (
	"context"
	"log/slog"
)

type backgroundHandler struct {
	slog.Handler
	metrics *Metrics
}

func (h *backgroundHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		worker := ""
		switch r.Message {
		case "AI usage period worker failed":
			worker = "ai_usage"
		case "CRM signal rule sweep failed":
			worker = "crm_signal_rules"
		}
		if worker != "" {
			h.metrics.logs.WithLabelValues(worker).Inc()
		}
	}
	return h.Handler.Handle(ctx, r)
}
func (h *backgroundHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &backgroundHandler{Handler: h.Handler.WithAttrs(attrs), metrics: h.metrics}
}
func (h *backgroundHandler) WithGroup(name string) slog.Handler {
	return &backgroundHandler{Handler: h.Handler.WithGroup(name), metrics: h.metrics}
}
