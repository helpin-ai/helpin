package service

import (
	"errors"
	"testing"
	"time"
)

func TestCustomerIOOutboxRetryClassification(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		retry bool
		delay time.Duration
	}{
		{"transport", &CustomerIODeliveryError{Err: errors.New("timeout")}, true, 0},
		{"request timeout", &CustomerIODeliveryError{StatusCode: 408}, true, 0},
		{"rate limited", &CustomerIODeliveryError{StatusCode: 429, RetryAfter: 30 * time.Second}, true, 30 * time.Second},
		{"server", &CustomerIODeliveryError{StatusCode: 503}, true, 0},
		{"bad request", &CustomerIODeliveryError{StatusCode: 400}, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retry, delay := customerIOOutboxRetry(tt.err)
			if retry != tt.retry || delay != tt.delay {
				t.Fatalf("got (%v,%s), want (%v,%s)", retry, delay, tt.retry, tt.delay)
			}
		})
	}
}
