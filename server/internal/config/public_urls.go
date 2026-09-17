package config

import (
	"fmt"
	"net/url"
	"strings"
)

// publicURL validates operator-owned addresses before exposing them to clients.
func publicURL(raw string, originOnly bool) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("public address must be an HTTP(S) URL without credentials, query or fragment")
	}
	if originOnly && u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("public origin cannot contain a path")
	}
	return strings.TrimRight(u.String(), "/"), nil
}
