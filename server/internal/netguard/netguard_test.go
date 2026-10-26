package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNonPublicIP(t *testing.T) {
	for _, raw := range []string{"10.0.0.1", "172.16.0.1", "192.168.1.1", "127.0.0.1", "::1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "fd00::1", "fe80::1", "224.0.0.1",
		"0.1.2.3", "198.18.0.1", "192.0.0.8", "240.0.0.1", "fec0::1", "::ffff:10.0.0.1",
		"64:ff9b::a00:1", "64:ff9b::7f00:1", "2002:a00:1::", "2002:7f00:1::1", "::10.0.0.1", "::169.254.169.254"} {
		if !IsNonPublicIP(net.ParseIP(raw)) {
			t.Errorf("%s should be non-public", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111", "64:ff9b::808:808", "2002:808:808::"} {
		if IsNonPublicIP(net.ParseIP(raw)) {
			t.Errorf("%s should be public", raw)
		}
	}
}

func TestIsLocalHostname(t *testing.T) {
	for host, want := range map[string]bool{"localhost": true, "api.localhost": true, "127.0.0.1": true, "::1": true, "example.com": false, "10.0.0.1": false} {
		if got := IsLocalHostname(host); got != want {
			t.Errorf("IsLocalHostname(%q) = %v", host, got)
		}
	}
}

func TestDialContextRefusesLoopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	guarded := &http.Client{Transport: &http.Transport{DialContext: DialContext(false)}}
	if _, err := guarded.Get(server.URL); !errors.Is(err, ErrNonPublicAddress) {
		t.Fatalf("loopback request through the guard: %v", err)
	}
	open := &http.Client{Transport: &http.Transport{DialContext: DialContext(true)}}
	resp, err := open.Get(server.URL)
	if err != nil {
		t.Fatalf("allowPrivate request: %v", err)
	}
	resp.Body.Close()
	if _, err := DialContext(false)(context.Background(), "tcp", "no-port"); err == nil {
		t.Fatal("address without a port accepted")
	}
}
