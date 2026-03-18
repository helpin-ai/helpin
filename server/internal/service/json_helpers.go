package service

import "encoding/json"

func parseJSONStringSlice(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
