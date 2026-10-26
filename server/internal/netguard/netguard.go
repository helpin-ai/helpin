// Package netguard keeps server-side HTTP requests to user-supplied URLs on
// the public Internet, so they cannot reach the server's own network.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// ErrNonPublicAddress means a host resolves to an address outside the
// public Internet.
var ErrNonPublicAddress = errors.New("host resolves to a non-public address")

var _sharedAddressSpace = mustCIDR("100.64.0.0/10")

func mustCIDR(raw string) *net.IPNet {
	_, network, err := net.ParseCIDR(raw)
	if err != nil {
		panic(err)
	}
	return network
}

// IsNonPublicIP reports whether ip is private, loopback, link-local,
// multicast, unspecified or in shared address space (100.64.0.0/10), which
// can reach provider infrastructure in some deployments.
func IsNonPublicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	return _sharedAddressSpace.Contains(ip)
}

// IsLocalHostname reports whether host names the local machine.
func IsLocalHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// DialContext returns a dial function that resolves the host itself and
// connects only to public addresses, unless allowPrivate is set. Checking
// the resolved address at dial time defeats DNS rebinding.
func DialContext(allowPrivate bool) func(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid address %q", address)
		}
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addrs) == 0 {
			return nil, fmt.Errorf("host %q could not be resolved", host)
		}
		if !allowPrivate {
			for _, addr := range addrs {
				if IsNonPublicIP(addr.IP) {
					return nil, ErrNonPublicAddress
				}
			}
		}
		var lastErr error
		for _, addr := range addrs {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		return nil, fmt.Errorf("host %q could not be reached: %w", host, lastErr)
	}
}
