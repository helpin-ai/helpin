// Package netguard keeps server-side HTTP requests to user-supplied URLs on
// the public Internet, so they cannot reach the server's own network.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
)

// ErrNonPublicAddress means a host resolves to an address outside the
// public Internet.
var ErrNonPublicAddress = errors.New("host resolves to a non-public address")

// _nonPublicRanges are special-purpose ranges that Go's IP helpers do not
// classify as private: shared address space, benchmarking, protocol
// assignments, documentation, reserved, and deprecated site-local IPv6.
var _nonPublicRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// _nat64 and _sixToFour embed an IPv4 address that the network may reach.
var (
	_nat64      = netip.MustParsePrefix("64:ff9b::/96")
	_nat64Local = netip.MustParsePrefix("64:ff9b:1::/48")
	_sixToFour  = netip.MustParsePrefix("2002::/16")
)

// IsNonPublicIP reports whether ip is private, loopback, link-local,
// multicast, unspecified or another special-purpose address, including IPv6
// forms that embed such an IPv4 address (NAT64, 6to4, IPv4-compatible).
func IsNonPublicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	return nonPublicAddr(addr.Unmap(), 0)
}

func nonPublicAddr(addr netip.Addr, depth int) bool {
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalMulticast() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() || addr.IsMulticast() {
		return true
	}
	for _, prefix := range _nonPublicRanges {
		if prefix.Contains(addr) {
			return true
		}
	}
	if addr.Is6() && depth == 0 {
		raw := addr.As16()
		switch {
		case _nat64.Contains(addr) || _nat64Local.Contains(addr):
			return nonPublicAddr(netip.AddrFrom4([4]byte(raw[12:16])), depth+1)
		case _sixToFour.Contains(addr):
			return nonPublicAddr(netip.AddrFrom4([4]byte(raw[2:6])), depth+1)
		case [12]byte(raw[:12]) == [12]byte{}:
			// IPv4-compatible ::a.b.c.d.
			return nonPublicAddr(netip.AddrFrom4([4]byte(raw[12:16])), depth+1)
		}
	}
	return false
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
