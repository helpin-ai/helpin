package worker

import (
	"encoding/json"
	"errors"
	"strings"
)

var ErrRunCancelled = errors.New("run was cancelled")

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

func looksLikeTestCommand(input json.RawMessage) bool {
	var params struct {
		Program string   `json:"program"`
		Args    []string `json:"args"`
		Command string   `json:"command"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(params.Command))
	if params.Program != "" {
		command = strings.ToLower(strings.TrimSpace(params.Program) + " " + strings.Join(params.Args, " "))
	}
	return strings.Contains(command, " test") || strings.HasPrefix(command, "test ") || strings.Contains(command, "go test") || strings.Contains(command, "npm test") || strings.Contains(command, "cargo test")
}

func toJSONString(value any) string {
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(data)
}

func toCompactJSONString(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
