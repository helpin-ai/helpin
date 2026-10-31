package handler

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestOIDCUserinfoVerifiedFlagCountsOnlyForTheSameAddress(t *testing.T) {
	cases := map[string]struct {
		userinfoEmail string
		wantLocation  string
		wantUsers     int64
	}{
		"same address, any case": {"SSO@Example.com", "http://localhost:5173/workspaces", 1},
		"different address":      {"someone-else@example.com", "http://localhost:5173/login?error=signin_failed", 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newOIDCHandlerFixture(t)
			f.provider.emailInIDToken, f.provider.omitVerified = true, true
			f.provider.userinfoEmail = tc.userinfoEmail
			cookies, state := f.start("")

			rec := f.callback(cookies, url.Values{"state": {state}, "code": {fakeOIDCCode}})

			if got := rec.Header().Get("Location"); got != tc.wantLocation {
				t.Fatalf("callback redirected to %q, want %q", got, tc.wantLocation)
			}
			if got := f.userCount(); got != tc.wantUsers {
				t.Fatalf("users = %d, want %d", got, tc.wantUsers)
			}
		})
	}
}

func TestOIDCDiscoveryDoesNotHoldALockWhileTheProviderHangs(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	gate := make(chan struct{})
	f.provider.set(func(p *fakeOIDCProvider) { p.discoveryGate = gate })

	var wg sync.WaitGroup
	slowErrs := make([]error, 3)
	for i := range slowErrs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, slowErrs[i] = f.handler.oidc.discover(context.Background())
		}()
	}
	waitFor(t, func() bool {
		f.provider.mu.Lock()
		defer f.provider.mu.Unlock()
		return f.provider.discoveryHits >= 1
	})

	// A request that ends while discovery hangs returns at once instead of
	// queueing behind the slow ones.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := f.handler.oidc.discover(ctx); err == nil {
		t.Fatal("discover() succeeded while the provider was hanging")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("a waiter took %s to give up, want it bound by its own context", elapsed)
	}

	close(gate)
	wg.Wait()
	for i, err := range slowErrs {
		if err != nil {
			t.Fatalf("shared discovery %d failed: %v", i, err)
		}
	}
	f.provider.mu.Lock()
	defer f.provider.mu.Unlock()
	if f.provider.discoveryHits != 1 {
		t.Fatalf("discovery requests = %d, want the callers to share one", f.provider.discoveryHits)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
