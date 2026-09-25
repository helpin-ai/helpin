package observability

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns a process-local registry with bounded, content-free labels.
type Metrics struct {
	translation     *translationMetrics
	registry        *prometheus.Registry
	requests        *prometheus.CounterVec
	latency         *prometheus.HistogramVec
	widget          *prometheus.CounterVec
	widgetLatency   *prometheus.HistogramVec
	decisions       *prometheus.CounterVec
	decisionLatency *prometheus.HistogramVec
	logs            *prometheus.CounterVec
}

// NewMetrics creates a registry independent of global Prometheus state.
func NewMetrics() *Metrics {
	m := &Metrics{
		registry:        prometheus.NewRegistry(),
		requests:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helpin_http_requests_total", Help: "Completed API requests by route template."}, []string{"method", "route", "status"}),
		latency:         prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "helpin_http_duration_seconds", Help: "API request duration.", Buckets: []float64{.01, .05, .1, .25, .5, 1, 2, 5, 15, 60}}, []string{"route"}),
		widget:          prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helpin_widget_events_total", Help: "Best-effort browser reported outcomes; not authoritative billing data."}, []string{"stage", "outcome", "browser"}),
		widgetLatency:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "helpin_widget_duration_seconds", Help: "Browser reported stage duration.", Buckets: []float64{.1, .5, 1, 2, 5, 15, 30, 60, 300, 600}}, []string{"stage", "outcome"}),
		decisions:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helpin_ai_decisions_total", Help: "AI decision and tagging outcomes."}, []string{"operation", "outcome"}),
		decisionLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "helpin_ai_duration_seconds", Help: "AI decision duration.", Buckets: []float64{.05, .1, .25, .5, 1, 2, 5, 15}}, []string{"operation"}),
		logs:            prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helpin_background_errors_total", Help: "Known background worker errors."}, []string{"worker"}),
	}
	m.registry.MustRegister(m.requests, m.latency, m.widget, m.widgetLatency, m.decisions, m.decisionLatency, m.logs, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	m.translation = newTranslationMetrics(m.registry)
	return m
}

// Handler exposes only this registry; mount on the internal metrics listener.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// HTTP measures matched route templates, never raw paths or query strings.
func (m *Metrics) HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			route := "unmatched"
			if ctx := chi.RouteContext(r.Context()); ctx != nil && ctx.RoutePattern() != "" {
				route = ctx.RoutePattern()
			}
			method := r.Method
			switch method {
			case "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD":
			default:
				method = "OTHER"
			}
			status := rw.Status()
			if status == 0 {
				status = 200
			}
			m.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
			m.latency.WithLabelValues(route).Observe(time.Since(start).Seconds())
		}()
		next.ServeHTTP(rw, r)
	})
}

// Widget records a validated, bounded browser event.
func (m *Metrics) Widget(stage, outcome, browser string, seconds float64) {
	m.widget.WithLabelValues(stage, outcome, browser).Inc()
	m.widgetLatency.WithLabelValues(stage, outcome).Observe(seconds)
}

// Decision records only internal constant operation/outcome names.
func (m *Metrics) Decision(operation, outcome string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.decisions.WithLabelValues(operation, outcome).Inc()
	m.decisionLatency.WithLabelValues(operation).Observe(elapsed.Seconds())
}

// BackgroundHandler counts known worker failures while retaining structured logging.
func (m *Metrics) BackgroundHandler(next slog.Handler) slog.Handler {
	return &backgroundHandler{Handler: next, metrics: m}
}
