package crawler

import "strings"

// ParseProxyURLs splits a comma-separated proxy URL string into a slice.
// Empty entries are ignored. Supports http, https, and socks5 proxy URLs.
//
// Example input: "http://user:pass@gate.decodo.com:10001,http://user:pass@gate.decodo.com:10000"
func ParseProxyURLs(commaSeparated string) []string {
	if strings.TrimSpace(commaSeparated) == "" {
		return nil
	}
	parts := strings.Split(commaSeparated, ",")
	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	return urls
}
