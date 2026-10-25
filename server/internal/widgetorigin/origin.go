// Package widgetorigin defines the browser origin boundary shared by widget HTTP
// and WebSocket requests. Installation keys identify a workspace, not permission
// to embed its widget on an arbitrary website.
package widgetorigin

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// subdomainPatternPrefix starts an allowlist entry that admits every subdomain
// of one domain over HTTPS, such as https://*.example.com.
const subdomainPatternPrefix = "https://*."

var errSubdomainPattern = errors.New(
	"subdomain patterns must look like https://*.example.com and use a domain you own")

// Reference identifies the installation and/or session used by a request.
// When multiple identifiers are supplied, they must identify the same installation.
type Reference struct {
	WidgetKey      string
	SessionToken   string
	InstallationID string
}

// Normalize accepts HTTP(S) origins and the exact local Tauri application origin.
func Normalize(raw string) (string, error) {
	// Tauri's custom protocol must remain distinct from its HTTP(S) variants.
	// No arbitrary custom schemes, hosts, ports, paths, or opaque origins.
	if strings.ToLower(raw) == "tauri://localhost" {
		return "tauri://localhost", nil
	}
	u, err := url.Parse(raw)
	if err != nil || raw != strings.TrimSpace(raw) || u == nil ||
		(u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" ||
		u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery ||
		u.Fragment != "" || strings.ContainsAny(u.Host, "* ,\\") || strings.Contains(raw, "#") {
		return "", fmt.Errorf("origin must be an HTTP(S) scheme and host or tauri://localhost")
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

// NormalizeEntry accepts one stored allowlist entry: "*", an exact origin that
// Normalize accepts, or an HTTPS subdomain pattern such as https://*.example.com.
// A pattern must sit below a registrable domain, so *.com or *.github.io is
// rejected rather than admitting unrelated sites.
func NormalizeEntry(raw string) (string, error) {
	if raw == "*" {
		return "*", nil
	}
	if !strings.Contains(raw, "*") {
		return Normalize(raw)
	}
	if len(raw) <= len(subdomainPatternPrefix) ||
		!strings.EqualFold(raw[:len(subdomainPatternPrefix)], subdomainPatternPrefix) {
		return "", errSubdomainPattern
	}
	base, err := Normalize("https://" + raw[len(subdomainPatternPrefix):])
	if err != nil {
		return "", errSubdomainPattern
	}
	host := strings.TrimPrefix(base, "https://")
	hostname := host
	if name, _, err := net.SplitHostPort(host); err == nil {
		hostname = name
	}
	if net.ParseIP(strings.Trim(hostname, "[]")) != nil || !strings.Contains(hostname, ".") {
		return "", errSubdomainPattern
	}
	if _, err := publicsuffix.EffectiveTLDPlusOne(hostname); err != nil {
		return "", errSubdomainPattern
	}
	return subdomainPatternPrefix + host, nil
}

// WebsiteDefaults returns the allowlist suggested by a workspace website: the
// site without a leading "www." and, when the site is on a registrable domain,
// an HTTPS pattern for its subdomains. The pattern never reaches above the
// website's own host, so a site on a shared platform covers only its own
// subdomains. It returns nil when the website is not an HTTP(S) host.
func WebsiteDefaults(website string) []string {
	raw := strings.TrimSpace(website)
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil
	}
	hostname := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host := hostname
	if port := u.Port(); port != "" {
		host = net.JoinHostPort(hostname, port)
	}
	site, err := Normalize(u.Scheme + "://" + host)
	if err != nil {
		return nil
	}
	defaults := []string{site}
	if u.Port() != "" {
		return defaults
	}
	if pattern, err := NormalizeEntry(subdomainPatternPrefix + hostname); err == nil {
		defaults = append(defaults, pattern)
	}
	return defaults
}

// Allowed requires an exact match or a subdomain pattern match unless the
// installation explicitly opts into all origins with "*". That policy also
// admits missing and opaque origins. Empty lists still deny access.
func Allowed(origin string, allowlist []string) bool {
	for _, entry := range allowlist {
		if entry == "*" {
			return true
		}
	}
	normalized, err := Normalize(origin)
	if err != nil {
		return false
	}
	for _, entry := range allowlist {
		if matchesSubdomainPattern(normalized, entry) {
			return true
		}
		candidate, err := Normalize(entry)
		if err == nil && normalized == candidate {
			return true
		}
	}
	return false
}

// matchesSubdomainPattern reports whether a normalized origin is a subdomain of
// a stored https://*.example.com entry on the same port. The domain itself is
// not a match; it needs its own entry.
func matchesSubdomainPattern(origin, entry string) bool {
	suffix, ok := strings.CutPrefix(entry, subdomainPatternPrefix)
	if !ok || suffix == "" {
		return false
	}
	host, ok := strings.CutPrefix(origin, "https://")
	return ok && len(host) > len(suffix)+1 && strings.HasSuffix(host, "."+suffix)
}
