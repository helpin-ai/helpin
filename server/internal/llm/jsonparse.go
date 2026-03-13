package llm

import (
	"encoding/json"
	"strings"
)

// UnmarshalResponse attempts to parse LLM response content as JSON, handling
// common LLM output quirks like markdown code fences and surrounding prose.
func UnmarshalResponse(content string, target any) error {
	// 1. Try direct unmarshal.
	if err := json.Unmarshal([]byte(content), target); err == nil {
		return nil
	}

	// 2. Strip markdown fences and retry.
	trimmed := trimJSONFences(content)
	if err := json.Unmarshal([]byte(trimmed), target); err == nil {
		return nil
	}

	// 3. Extract first balanced JSON object or array and retry.
	for _, candidate := range []string{
		extractJSON(content),
		extractJSON(trimmed),
	} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(candidate), target); err == nil {
			return nil
		}
	}

	// 4. Fall back to the trimmed version for error reporting.
	return json.Unmarshal([]byte(trimmed), target)
}

// trimJSONFences strips ```json ... ``` wrappers from LLM output.
func trimJSONFences(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 {
		return trimmed
	}
	lines = lines[1:]
	if lines[len(lines)-1] == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// extractJSON finds the first balanced JSON object ({...}) or array ([...])
// in raw text, tracking depth and string escaping.
func extractJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	start := -1
	depth := 0
	inString := false
	escaped := false
	var openChar, closeChar rune

	for idx, r := range trimmed {
		if start == -1 {
			if r == '{' || r == '[' {
				start = idx
				depth = 1
				openChar = r
				if r == '{' {
					closeChar = '}'
				} else {
					closeChar = ']'
				}
			}
			continue
		}

		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch r {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch r {
		case '"':
			inString = true
		case openChar:
			depth++
		case closeChar:
			depth--
			if depth == 0 {
				return strings.TrimSpace(trimmed[start : idx+1])
			}
		}
	}

	return ""
}
