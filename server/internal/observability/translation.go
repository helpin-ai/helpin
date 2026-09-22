package observability

import (
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type translationMetrics struct {
	queue    *prometheus.GaugeVec
	workflow *prometheus.HistogramVec
	events   *prometheus.CounterVec
	attempts *prometheus.CounterVec
	duration *prometheus.HistogramVec
	tokens   *prometheus.CounterVec
	cost     *prometheus.CounterVec
	coverage *prometheus.CounterVec
}

func newTranslationMetrics(registry *prometheus.Registry) *translationMetrics {
	m := &translationMetrics{
		queue:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "helpin_translation_queue_messages", Help: "Durable translation/send work by queue and state."}, []string{"queue", "state"}),
		workflow: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "helpin_translation_workflow_seconds", Help: "Complete translation workflow duration.", Buckets: []float64{.1, .5, 1, 2, 5, 10, 25, 60, 120}}, []string{"purpose"}),
		events:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helpin_translation_events_total", Help: "Translation workflow outcomes; cache hits do not call a provider."}, []string{"direction", "outcome"}),
		attempts: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helpin_translation_provider_attempts_total", Help: "Actual translation and sampled review provider calls, including retries."}, []string{"stage", "provider", "model", "outcome"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "helpin_translation_provider_duration_seconds", Help: "Translation provider attempt latency.", Buckets: []float64{.1, .25, .5, 1, 2, 5, 10, 15, 25, 60}}, []string{"stage", "provider", "model"}),
		tokens:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helpin_translation_tokens_total", Help: "Provider-reported normalized tokens; kinds are disjoint."}, []string{"stage", "provider", "model", "kind"}),
		cost:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helpin_translation_estimated_provider_cost_usd_total", Help: "Estimated provider spend using immutable admission rates; excludes calls without rates, not customer charges."}, []string{"stage", "provider", "model"}),
		coverage: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helpin_translation_usage_measurements_total", Help: "Coverage of token and price measurements; missing prices are not zero cost."}, []string{"stage", "measurement"}),
	}
	registry.MustRegister(m.queue, m.workflow, m.events, m.attempts, m.duration, m.tokens, m.cost, m.coverage)
	return m
}

// TranslationEvent accepts only internal constant directions and outcomes.
func (m *Metrics) TranslationEvent(direction, outcome string) {
	if m == nil {
		return
	}
	m.translation.events.WithLabelValues(direction, outcome).Inc()
}

// TranslationAttempt must be called once for each real provider request. Provider
// and model come from server-owned routing, never user text or conversation IDs.
func (m *Metrics) TranslationAttempt(stage, provider, model, outcome string, elapsed time.Duration, telemetry *aiusage.TokenTelemetry, rates aiusage.TokenRates) {
	if m == nil {
		return
	}
	m.translation.attempts.WithLabelValues(stage, provider, model, outcome).Inc()
	m.translation.duration.WithLabelValues(stage, provider, model).Observe(elapsed.Seconds())
	if telemetry == nil {
		m.translation.coverage.WithLabelValues(stage, "tokens_missing").Inc()
		return
	}
	n, err := aiusage.NormalizeTokens(*telemetry)
	if err != nil {
		m.translation.coverage.WithLabelValues(stage, "tokens_invalid").Inc()
		return
	}
	m.translation.coverage.WithLabelValues(stage, "tokens_reported").Inc()
	for kind, count := range map[string]int64{"input": n.UncachedInputTokens, "cache_read": n.CacheReadTokens, "cache_write": n.CacheWriteTokens, "output": n.OutputTokens, "reasoning": n.ReasoningTokens} {
		m.translation.tokens.WithLabelValues(stage, provider, model, kind).Add(float64(count))
	}
	if rates.InputMicrousdPerMillion <= 0 || rates.OutputMicrousdPerMillion <= 0 {
		m.translation.coverage.WithLabelValues(stage, "price_missing").Inc()
		return
	}
	m.translation.coverage.WithLabelValues(stage, "price_available").Inc()
	usd := (float64(n.UncachedInputTokens)*float64(rates.InputMicrousdPerMillion) + float64(n.CacheReadTokens)*float64(rates.CacheReadMicrousdPerMillion) + float64(n.CacheWriteTokens)*float64(rates.CacheWriteMicrousdPerMillion) + float64(n.OutputTokens+n.ReasoningTokens)*float64(rates.OutputMicrousdPerMillion)) / 1e12
	m.translation.cost.WithLabelValues(stage, provider, model).Add(usd)
}

func (m *Metrics) TranslationQueue(queue, state string, count int64) {
	if m != nil {
		m.translation.queue.WithLabelValues(queue, state).Set(float64(count))
	}
}
func (m *Metrics) TranslationWorkflow(purpose string, elapsed time.Duration) {
	if m != nil {
		m.translation.workflow.WithLabelValues(purpose).Observe(elapsed.Seconds())
	}
}
