package requestmeta

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPContextKey struct{}

type ClientIP struct {
	Addr   netip.Addr
	Source string
}

func WithClientIP(ctx context.Context, clientIP ClientIP) context.Context {
	if !clientIP.Addr.IsValid() {
		return ctx
	}
	clientIP.Addr = clientIP.Addr.Unmap()
	return context.WithValue(ctx, clientIPContextKey{}, clientIP)
}

func ClientIPFromContext(ctx context.Context) (ClientIP, bool) {
	clientIP, ok := ctx.Value(clientIPContextKey{}).(ClientIP)
	if !ok || !clientIP.Addr.IsValid() {
		return ClientIP{}, false
	}
	clientIP.Addr = clientIP.Addr.Unmap()
	return clientIP, true
}

func ExtractClientIP(r *http.Request) (ClientIP, bool) {
	candidates := defaultIPCandidates(r)
	if hasCloudflareHeaders(r) {
		candidates = cloudflareIPCandidates(r)
	}

	for _, candidate := range candidates {
		for _, value := range candidate.values {
			addr, ok := parseIP(value)
			if ok {
				return ClientIP{Addr: addr, Source: candidate.source}, true
			}
		}
	}

	return ClientIP{}, false
}

func defaultIPCandidates(r *http.Request) []struct {
	source string
	values []string
} {
	return []struct {
		source string
		values []string
	}{
		{source: "true-client-ip", values: headerCandidates(r.Header.Get("True-Client-IP"))},
		{source: "x-real-ip", values: headerCandidates(r.Header.Get("X-Real-IP"))},
		{source: "forwarded", values: forwardedCandidates(r.Header.Get("Forwarded"))},
		{source: "x-forwarded-for", values: headerCandidates(r.Header.Get("X-Forwarded-For"))},
		{source: "remote-addr", values: []string{r.RemoteAddr}},
	}
}

func cloudflareIPCandidates(r *http.Request) []struct {
	source string
	values []string
} {
	return []struct {
		source string
		values []string
	}{
		{source: "cf-connecting-ip", values: headerCandidates(r.Header.Get("CF-Connecting-IP"))},
		{source: "x-original-forwarded-for", values: headerCandidates(r.Header.Get("X-Original-Forwarded-For"))},
		{source: "true-client-ip", values: headerCandidates(r.Header.Get("True-Client-IP"))},
		{source: "forwarded", values: forwardedCandidates(r.Header.Get("Forwarded"))},
		{source: "x-forwarded-for", values: headerCandidates(r.Header.Get("X-Forwarded-For"))},
		{source: "x-real-ip", values: headerCandidates(r.Header.Get("X-Real-IP"))},
		{source: "remote-addr", values: []string{r.RemoteAddr}},
	}
}

func hasCloudflareHeaders(r *http.Request) bool {
	for _, name := range []string{
		"CF-Connecting-IP",
		"CF-Ray",
		"CF-Visitor",
		"CF-IPCountry",
		"CDN-Loop",
		"X-Original-Forwarded-For",
	} {
		if strings.TrimSpace(r.Header.Get(name)) != "" {
			return true
		}
	}
	return false
}

func IsPublicIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || !addr.IsGlobalUnicast() {
		return false
	}
	if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	return true
}

func headerCandidates(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func forwardedCandidates(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	entries := strings.Split(raw, ",")
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		for _, segment := range strings.Split(entry, ";") {
			segment = strings.TrimSpace(segment)
			if !strings.HasPrefix(strings.ToLower(segment), "for=") {
				continue
			}
			value := strings.TrimSpace(segment[4:])
			value = strings.Trim(value, "\"")
			value = strings.TrimPrefix(value, "[")
			value = strings.TrimSuffix(value, "]")
			if value != "" {
				values = append(values, value)
			}
		}
	}
	return values
}

func parseIP(raw string) (netip.Addr, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return netip.Addr{}, false
	}

	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}

	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")

	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}
