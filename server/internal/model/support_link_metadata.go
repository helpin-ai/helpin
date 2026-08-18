package model

import (
	"encoding/json"
	"strings"
)

// StripSupportLinkSecurityMetadata removes provider verdicts from public widget metadata.
func StripSupportLinkSecurityMetadata(metadata string) string {
	if strings.TrimSpace(metadata) == "" {
		return metadata
	}
	values := map[string]any{}
	if err := json.Unmarshal([]byte(metadata), &values); err != nil {
		return "{}"
	}
	delete(values, "link_security")
	encoded, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
