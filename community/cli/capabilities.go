package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var errCapabilitiesUnsupported = errors.New("this Helpin version does not report capabilities")

// capabilityStatus mirrors the server's instance capability response.
type capabilityStatus struct {
	Key      string `json:"key"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
	Required bool   `json:"required"`
	Action   *struct {
		Label string `json:"label"`
	} `json:"action"`
}

var capabilityNames = map[string]string{
	"ai_chat":               "AI chat",
	"ai_embeddings":         "AI knowledge search",
	"email_outbound":        "Application email",
	"support_widget":        "Support widget",
	"support_email_inbound": "Inbound support email",
	"github":                "GitHub",
	"object_storage":        "Object storage",
	"workers":               "Background workers",
	"meeting_capture":       "Meeting capture",
	"google_workspace":      "Google (Gmail and Calendar)",
}

// instanceCapabilities asks the local API through the loopback ingress. The
// internal API secret authenticates the request; it never leaves this host.
func instanceCapabilities(client *http.Client, values map[string]string) ([]capabilityStatus, error) {
	secret := plainEnv(values["INTERNAL_API_SECRET"])
	if secret == "" {
		return nil, errors.New("INTERNAL_API_SECRET is missing from .env")
	}
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+values["DASHBOARD_PORT"]+"/api/instance/capabilities", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+secret)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return nil, errCapabilitiesUnsupported
	case http.StatusUnauthorized:
		return nil, errors.New("the API rejected INTERNAL_API_SECRET; restart services after changing .env")
	default:
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	var body struct {
		Capabilities []capabilityStatus `json:"capabilities"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil {
		return nil, err
	}
	return body.Capabilities, nil
}

// reportCapabilities prints each capability with its next action and returns
// the required capabilities that are not ready. Optional integrations that
// need setup are reported without failing doctor.
func (a *app) reportCapabilities(client *http.Client, values map[string]string) []string {
	fmt.Fprintln(a.out, "\nCapabilities:")
	capabilities, err := instanceCapabilities(client, values)
	if errors.Is(err, errCapabilitiesUnsupported) {
		fmt.Fprintln(a.out, "- Skipped:", err)
		return nil
	}
	if err != nil {
		fmt.Fprintf(a.out, "✗ Capability status: %v\n", err)
		return []string{"Capability status"}
	}
	var failures []string
	for _, c := range capabilities {
		name := capabilityNames[c.Key]
		if name == "" {
			name = c.Key
		}
		symbol := "!"
		switch {
		case c.Status == "ready":
			symbol = "✓"
		case c.Status == "unavailable":
			symbol = "-"
		case c.Required && c.Status == "needs_setup":
			symbol = "✗"
			failures = append(failures, name)
		}
		fmt.Fprintf(a.out, "%s %s: %s — %s\n", symbol, name, strings.ReplaceAll(c.Status, "_", " "), c.Detail)
		if c.Action != nil && c.Action.Label != "" && c.Status != "ready" {
			fmt.Fprintf(a.out, "    Next: %s\n", c.Action.Label)
		}
	}
	return failures
}
