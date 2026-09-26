//go:build !ee

package observability

import "testing"

func TestCommunityHasNoDefaultTelemetryEndpoint(t *testing.T) {
	if defaultSentryDSN != "" {
		t.Fatal("Community must not have a default error-reporting endpoint")
	}
}
