package service

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// PublicBaseURLBlockedReason explains why an external service (named by
// sender, for example "GitHub" or "Recall") cannot deliver webhooks to
// appBaseURL, or returns "" when it is an https URL on a public hostname or
// public IP address.
func PublicBaseURLBlockedReason(sender, appBaseURL string) string {
	value := strings.TrimSpace(appBaseURL)
	if publicHTTPSReachable(value) {
		return ""
	}
	shown := value
	if shown == "" {
		shown = "not set"
	}
	return fmt.Sprintf("%s must reach this server to deliver events. Set APP_BASE_URL to a public https address (currently %s).", sender, shown)
}

// publicHTTPSReachable reports whether a service on the internet can reach
// rawURL: https on a public hostname or a public IP address.
func publicHTTPSReachable(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast())
	}
	if host == "localhost" || !strings.Contains(host, ".") {
		return false
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".lan", ".home.arpa"} {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	return true
}
