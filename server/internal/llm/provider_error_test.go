package llm

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestProviderErrorRetryability(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "rate limit", err: &ProviderError{StatusCode: http.StatusTooManyRequests}, want: true},
		{name: "request timeout", err: &ProviderError{StatusCode: http.StatusRequestTimeout}, want: true},
		{name: "server error", err: &ProviderError{StatusCode: http.StatusBadGateway}, want: true},
		{name: "upstream credits", err: &ProviderError{StatusCode: http.StatusPaymentRequired, Err: ErrInsufficientCredits}, want: true},
		{name: "provider missing", err: &ProviderError{Kind: ProviderErrorUnavailable}, want: true},
		{name: "bad request", err: &ProviderError{StatusCode: http.StatusBadRequest}, want: false},
		{name: "structured output has no compatible endpoint", err: &ProviderError{Provider: "openrouter", StatusCode: http.StatusNotFound, Message: `{"error":{"message":"No endpoints found that support the requested parameters"}}`}, want: true},
		{name: "missing model route", err: &ProviderError{Provider: "openrouter", StatusCode: http.StatusNotFound, Message: `{"error":{"message":"No endpoints found for this model"}}`}, want: true},
		{name: "unrelated not found", err: &ProviderError{Provider: "openrouter", StatusCode: http.StatusNotFound, Message: "resource not found"}, want: false},
		{name: "caller cancelled", err: &ProviderError{Err: context.Canceled}, want: false},
		{name: "caller deadline", err: &ProviderError{Err: context.DeadlineExceeded}, want: false},
		{name: "ordinary error", err: errors.New("bad output"), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsRetryableProviderError(test.err); got != test.want {
				t.Fatalf("IsRetryableProviderError() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRouterReturnsRetryableTypedErrorForMissingProvider(t *testing.T) {
	router := NewRouter("openrouter", map[string]Provider{}, "", nil)
	_, err := router.ChatCompletion(context.Background(), ChatRequest{Provider: "openrouter"})
	if err == nil {
		t.Fatal("ChatCompletion() error = nil")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != ProviderErrorUnavailable {
		t.Fatalf("ChatCompletion() error = %#v, want unavailable ProviderError", err)
	}
	if !IsRetryableProviderError(err) {
		t.Fatal("missing preferred provider should allow declared fallback")
	}
}
