package observability

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

const (
	defaultSentryDSN              = "https://313588e74d7c61c42a9a16c3f7ec019e@sen.usrmvn.com/6"
	defaultSentryTracesSampleRate = 1.0
)

func InitSentry(service string) error {
	tracesSampleRate := defaultSentryTracesSampleRate
	if raw := strings.TrimSpace(os.Getenv("SENTRY_TRACES_SAMPLE_RATE")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("parse SENTRY_TRACES_SAMPLE_RATE: %w", err)
		}
		tracesSampleRate = parsed
	}

	return sentry.Init(sentry.ClientOptions{
		Dsn:              firstNonEmpty(strings.TrimSpace(os.Getenv("SENTRY_DSN")), defaultSentryDSN),
		TracesSampleRate: tracesSampleRate,
		Environment:      firstNonEmpty(strings.TrimSpace(os.Getenv("SENTRY_ENVIRONMENT")), strings.TrimSpace(os.Getenv("APP_ENV")), strings.TrimSpace(os.Getenv("GO_ENV"))),
		Release:          strings.TrimSpace(os.Getenv("SENTRY_RELEASE")),
		AttachStacktrace: true,
		Tags: map[string]string{
			"service": service,
		},
	})
}

func Flush(timeout time.Duration) bool {
	return sentry.Flush(timeout)
}

func CaptureException(err error) *sentry.EventID {
	if err == nil {
		return nil
	}
	return sentry.CaptureException(err)
}

func CaptureMessage(message string) *sentry.EventID {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	return sentry.CaptureMessage(message)
}

func CaptureRecovered(recovered any) *sentry.EventID {
	if recovered == nil {
		return nil
	}
	return sentry.CurrentHub().Recover(recovered)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
