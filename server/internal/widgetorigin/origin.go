// Package widgetorigin defines the browser origin boundary shared by widget HTTP
// and WebSocket requests. Installation keys identify a workspace, not permission
// to embed its widget on an arbitrary website.
package widgetorigin

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Reference identifies the installation and/or session used by a request.
// When multiple identifiers are supplied, they must identify the same installation.
type Reference struct {
	WidgetKey      string
	SessionToken   string
	InstallationID string
}

// Normalize accepts a single HTTP origin, including an optional non-default port.
func Normalize(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || raw != strings.TrimSpace(raw) || u == nil ||
		(u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" ||
		u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery ||
		u.Fragment != "" || strings.ContainsAny(u.Host, "* ,\\") || strings.Contains(raw, "#") {
		return "", fmt.Errorf("origin must contain only an HTTP scheme and host")
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return u.Scheme + "://" + host, nil
}

// Allowed requires an exact origin match. Empty lists and missing/null origins
// deny access; neither same-origin requests nor identity report mode bypass it.
func Allowed(origin string, allowlist []string) bool {
	normalized, err := Normalize(origin)
	if err != nil {
		return false
	}
	for _, entry := range allowlist {
		candidate, err := Normalize(entry)
		if err == nil && normalized == candidate {
			return true
		}
	}
	return false
}
