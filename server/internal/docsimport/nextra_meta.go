package docsimport

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// NextraMetaEntry represents one key in a Nextra _meta file.
type NextraMetaEntry struct {
	Key     string
	Title   string
	Type    string         // "page", "menu", "separator", etc.
	Href    string         // external link
	Display string         // "hidden", "children", "normal"
	Items   []NextraMetaEntry
	Theme   map[string]any // theme overrides like {"sidebar": false}
}

// ParseNextraMeta parses a Nextra _meta.json, _meta.js, or _meta.ts
// file into an ordered list of entries. It uses static parsing only —
// no JavaScript is executed. Dynamic values produce warnings.
func ParseNextraMeta(path string, data []byte) ([]NextraMetaEntry, []Warning) {
	if len(data) == 0 {
		return nil, []Warning{{
			Type:    "meta_parse_error",
			Message: fmt.Sprintf("%s: empty file", path),
		}}
	}

	var jsonData []byte
	isJSON := strings.HasSuffix(path, ".json")

	if isJSON {
		jsonData = data
	} else {
		// Extract the object from JS/TS export.
		extracted, ok := extractJSObject(data)
		if !ok {
			return nil, []Warning{{
				Type:    "meta_parse_fallback",
				Message: fmt.Sprintf("%s: could not extract static object from JS/TS meta file", path),
			}}
		}
		jsonData = extracted
	}

	// Clean up the JSON for lenient parsing.
	jsonData = cleanJSON(jsonData)

	entries, err := parseOrderedJSON(jsonData)
	if err != nil {
		return nil, []Warning{{
			Type:    "meta_parse_error",
			Message: fmt.Sprintf("%s: %v", path, err),
		}}
	}

	return entries, nil
}

// extractJSObject extracts the object literal after `export default`
// or `module.exports =` from a JS/TS file. It strips import lines,
// `as const`, and `satisfies ...` suffixes.
func extractJSObject(data []byte) ([]byte, bool) {
	s := string(data)

	// Remove single-line comments.
	s = regexp.MustCompile(`//[^\n]*`).ReplaceAllString(s, "")
	// Remove multi-line comments.
	s = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(s, "")

	// Remove import lines.
	s = regexp.MustCompile(`(?m)^import\s+.*$`).ReplaceAllString(s, "")

	// Strategy: find the first `{...}` object in the file, trying:
	// 1. `export default { ... }` — direct object export
	// 2. `module.exports = { ... }` — CommonJS
	// 3. `const/let/var NAME = { ... }; export default NAME;` — variable then export
	// For case 3, `export default NAME` doesn't have `{` after it,
	// so we fall through to the variable assignment pattern.

	var rest string
	found := false

	// Try `export default {` first.
	exportRe := regexp.MustCompile(`export\s+default\s+`)
	if loc := exportRe.FindStringIndex(s); loc != nil {
		after := strings.TrimSpace(s[loc[1]:])
		if len(after) > 0 && after[0] == '{' {
			rest = s[loc[1]:]
			found = true
		}
	}

	// Try `module.exports = {`.
	if !found {
		moduleRe := regexp.MustCompile(`module\.exports\s*=\s*`)
		if loc := moduleRe.FindStringIndex(s); loc != nil {
			after := strings.TrimSpace(s[loc[1]:])
			if len(after) > 0 && after[0] == '{' {
				rest = s[loc[1]:]
				found = true
			}
		}
	}

	// Try `const/let/var NAME = {`.
	if !found {
		varRe := regexp.MustCompile(`(?:const|let|var)\s+\w+\s*=\s*`)
		if loc := varRe.FindStringIndex(s); loc != nil {
			after := strings.TrimSpace(s[loc[1]:])
			if len(after) > 0 && after[0] == '{' {
				rest = s[loc[1]:]
				found = true
			}
		}
	}

	if !found {
		return nil, false
	}

	// Find the matching closing brace.
	braceDepth := 0
	objEnd := -1
	inString := false
	var strChar byte

	for i := 0; i < len(rest); i++ {
		ch := rest[i]
		if inString {
			if ch == '\\' {
				i++ // skip escaped char
				continue
			}
			if ch == strChar {
				inString = false
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			inString = true
			strChar = ch
			continue
		}
		if ch == '{' {
			braceDepth++
		} else if ch == '}' {
			braceDepth--
			if braceDepth == 0 {
				objEnd = i + 1
				break
			}
		}
	}

	if objEnd < 0 {
		return nil, false
	}

	obj := rest[:objEnd]

	// Strip trailing `as const`, `satisfies Meta`, etc.
	obj = strings.TrimSpace(obj)

	return []byte(obj), true
}

// cleanJSON makes lenient JSON parseable: removes trailing commas,
// quotes unquoted keys, normalises single quotes to double quotes.
func cleanJSON(data []byte) []byte {
	s := string(data)

	// Quote unquoted keys: word chars at the start of a key position.
	// Match: beginning-of-object or comma, then whitespace, then bare key, then colon.
	s = regexp.MustCompile(`(?m)([\{,]\s*)([a-zA-Z_$][a-zA-Z0-9_$]*)\s*:`).
		ReplaceAllString(s, `$1"$2":`)

	// Replace single-quoted strings with double-quoted.
	s = replaceSingleQuotes(s)

	// Remove trailing commas before } or ].
	s = regexp.MustCompile(`,\s*([}\]])`).ReplaceAllString(s, "$1")

	return []byte(s)
}

// replaceSingleQuotes converts single-quoted strings to double-quoted.
func replaceSingleQuotes(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	inDouble := false
	inSingle := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\\' && i+1 < len(s) {
			result.WriteByte(ch)
			i++
			result.WriteByte(s[i])
			continue
		}
		if !inSingle && ch == '"' {
			inDouble = !inDouble
			result.WriteByte(ch)
			continue
		}
		if !inDouble && ch == '\'' {
			if !inSingle {
				inSingle = true
				result.WriteByte('"')
			} else {
				inSingle = false
				result.WriteByte('"')
			}
			continue
		}
		// Escape double quotes inside single-quoted strings.
		if inSingle && ch == '"' {
			result.WriteString(`\"`)
			continue
		}
		result.WriteByte(ch)
	}
	return result.String()
}

// parseOrderedJSON parses a JSON object preserving key order.
// Values can be strings or nested objects.
func parseOrderedJSON(data []byte) ([]NextraMetaEntry, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil, fmt.Errorf("empty JSON")
	}

	// Use json.Decoder to preserve order.
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()

	// Expect opening brace.
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("expected '{', got %v", tok)
	}

	var entries []NextraMetaEntry
	for dec.More() {
		// Read key.
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("read key: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %T", keyTok)
		}

		// Read value — could be string or object.
		var rawValue json.RawMessage
		if err := dec.Decode(&rawValue); err != nil {
			return nil, fmt.Errorf("read value for %q: %w", key, err)
		}

		entry := NextraMetaEntry{Key: key}
		if err := parseMetaValue(rawValue, &entry); err != nil {
			return nil, fmt.Errorf("parse value for %q: %w", key, err)
		}
		entries = append(entries, entry)
	}

	// Expect closing brace.
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("parse closing: %w", err)
	}

	return entries, nil
}

// parseMetaValue populates a NextraMetaEntry from a raw JSON value.
func parseMetaValue(raw json.RawMessage, entry *NextraMetaEntry) error {
	trimmed := strings.TrimSpace(string(raw))

	// String value: just the title.
	if strings.HasPrefix(trimmed, `"`) {
		var title string
		if err := json.Unmarshal(raw, &title); err != nil {
			return err
		}
		entry.Title = title
		return nil
	}

	// Object value: parse known fields.
	if strings.HasPrefix(trimmed, "{") {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return err
		}

		if v, ok := obj["title"]; ok {
			var title string
			if err := json.Unmarshal(v, &title); err == nil {
				entry.Title = title
			}
		}
		if v, ok := obj["type"]; ok {
			var typ string
			if err := json.Unmarshal(v, &typ); err == nil {
				entry.Type = typ
			}
		}
		if v, ok := obj["href"]; ok {
			var href string
			if err := json.Unmarshal(v, &href); err == nil {
				entry.Href = href
			}
		}
		if v, ok := obj["display"]; ok {
			var display string
			if err := json.Unmarshal(v, &display); err == nil {
				entry.Display = display
			}
		}
		if v, ok := obj["theme"]; ok {
			var theme map[string]any
			if err := json.Unmarshal(v, &theme); err == nil {
				entry.Theme = theme
			}
		}
		if v, ok := obj["items"]; ok {
			var items map[string]json.RawMessage
			if err := json.Unmarshal(v, &items); err == nil {
				for k, iv := range items {
					sub := NextraMetaEntry{Key: k}
					if err := parseMetaValue(iv, &sub); err == nil {
						entry.Items = append(entry.Items, sub)
					}
				}
			}
		}

		// If title is still empty, use the key.
		if entry.Title == "" {
			entry.Title = entry.Key
		}

		return nil
	}

	// Fallback: use key as title.
	entry.Title = entry.Key
	return nil
}
