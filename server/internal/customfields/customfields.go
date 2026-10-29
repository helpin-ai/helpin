// Package customfields validates, parses and displays the typed custom fields
// workspaces define for CRM records and support conversations. Values live in
// JSON maps on the records; a definition gives a key a label and a type.
package customfields

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Field types.
const (
	TypeText        = "text"
	TypeLongText    = "long_text"
	TypeNumber      = "number"
	TypeDate        = "date"
	TypeBoolean     = "boolean"
	TypeSelect      = "select"
	TypeMultiSelect = "multi_select"
)

// Types lists every field type in display order.
var Types = []string{TypeText, TypeLongText, TypeNumber, TypeDate, TypeBoolean, TypeSelect, TypeMultiSelect}

// Limits.
const (
	MaxTextLength     = 500
	MaxLongTextLength = 10000
	MaxOptions        = 100
	MaxOptionLabel    = 80
	MaxOptionValue    = 100
	DateLayout        = "2006-01-02"
)

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// ValidKey reports whether key may name a field or an option value.
func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// ValidType reports whether fieldType is a known field type.
func ValidType(fieldType string) bool { return slices.Contains(Types, fieldType) }

// Option is one choice of a select field. Value is stored on records and
// never changes; Label is shown and may change.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Definition is what validation needs to know about a field.
type Definition struct {
	Key     string
	Label   string
	Type    string
	Options []Option
}

// ErrInvalid wraps every validation failure; the message is safe to show.
var ErrInvalid = errors.New("invalid field value")

func invalid(def Definition, format string, args ...any) error {
	return fmt.Errorf("%w: %s %s", ErrInvalid, DisplayLabel(def), fmt.Sprintf(format, args...))
}

// DisplayLabel is the label, or the key written out when there is none.
func DisplayLabel(def Definition) string {
	if label := strings.TrimSpace(def.Label); label != "" {
		return label
	}
	return Humanize(def.Key)
}

// Humanize turns a key such as invoice_number into "Invoice number".
func Humanize(key string) string {
	text := strings.TrimSpace(strings.ReplaceAll(key, "_", " "))
	if text == "" {
		return ""
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

// NormalizeOptions checks a select field's options: unique values (ignoring
// case), labels defaulting to the value, humanized when it is key-like, at most MaxOptions.
func NormalizeOptions(options []Option) ([]Option, error) {
	if len(options) > MaxOptions {
		return nil, fmt.Errorf("%w: a field can have up to %d options", ErrInvalid, MaxOptions)
	}
	result := make([]Option, 0, len(options))
	seen := map[string]bool{}
	for _, option := range options {
		// Values match data already stored, so any visible text is allowed.
		value := strings.Join(strings.Fields(option.Value), " ")
		if value == "" || utf8.RuneCountInString(value) > MaxOptionValue || strings.ContainsFunc(value, unicode.IsControl) {
			return nil, fmt.Errorf("%w: option values need 1 to %d characters", ErrInvalid, MaxOptionValue)
		}
		if seen[strings.ToLower(value)] {
			return nil, fmt.Errorf("%w: option %q is listed twice", ErrInvalid, value)
		}
		seen[strings.ToLower(value)] = true
		label := strings.Join(strings.Fields(option.Label), " ")
		if label == "" && ValidKey(value) {
			label = Humanize(value)
		} else if label == "" {
			label = value
		}
		if utf8.RuneCountInString(label) > MaxOptionLabel {
			return nil, fmt.Errorf("%w: option labels can be at most %d characters", ErrInvalid, MaxOptionLabel)
		}
		result = append(result, Option{Value: value, Label: label})
	}
	return result, nil
}

// Validate checks a value a person entered and returns its canonical JSON
// form: strings for text, select and dates (YYYY-MM-DD), float64 for numbers,
// bool for yes/no, []string for multi-select. A nil result means "clear".
func Validate(def Definition, raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	switch def.Type {
	case TypeText, TypeLongText:
		text, ok := raw.(string)
		if !ok {
			return nil, invalid(def, "must be text")
		}
		text = strings.TrimSpace(text)
		if def.Type == TypeText {
			text = strings.Join(strings.Fields(text), " ")
		}
		if text == "" {
			return nil, nil
		}
		limit := MaxTextLength
		if def.Type == TypeLongText {
			limit = MaxLongTextLength
		}
		if utf8.RuneCountInString(text) > limit {
			return nil, invalid(def, "can be at most %d characters", limit)
		}
		return text, nil
	case TypeNumber:
		number, ok := toNumber(raw)
		if !ok {
			return nil, invalid(def, "must be a number")
		}
		return number, nil
	case TypeDate:
		text, ok := raw.(string)
		if !ok {
			return nil, invalid(def, "must be a date")
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, nil
		}
		if _, err := time.Parse(DateLayout, text); err != nil {
			return nil, invalid(def, "must be a date like 2026-10-29")
		}
		return text, nil
	case TypeBoolean:
		value, ok := raw.(bool)
		if !ok {
			return nil, invalid(def, "must be yes or no")
		}
		return value, nil
	case TypeSelect:
		text, ok := raw.(string)
		if !ok {
			return nil, invalid(def, "must be one of its options")
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, nil
		}
		if !hasOption(def, text) {
			return nil, invalid(def, "has no option %q", text)
		}
		return text, nil
	case TypeMultiSelect:
		values, ok := toStrings(raw)
		if !ok {
			return nil, invalid(def, "must be a list of its options")
		}
		result := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || slices.Contains(result, value) {
				continue
			}
			if !hasOption(def, value) {
				return nil, invalid(def, "has no option %q", value)
			}
			result = append(result, value)
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil
	}
	return nil, invalid(def, "has an unknown type")
}

// Fits reports whether a stored value matches the definition, as read
// leniently: integrations may store anything, and values that do not fit
// count as empty.
func Fits(def Definition, stored any) bool {
	if stored == nil {
		return true
	}
	switch def.Type {
	case TypeText, TypeLongText:
		_, ok := stored.(string)
		return ok
	case TypeNumber:
		_, ok := toNumber(stored)
		return ok
	case TypeDate:
		text, ok := stored.(string)
		if !ok {
			return false
		}
		// Exact form only, as the date filters read it.
		_, err := time.Parse(DateLayout, text)
		return err == nil
	case TypeBoolean:
		_, ok := stored.(bool)
		return ok
	case TypeSelect:
		text, ok := stored.(string)
		return ok && hasOption(def, text)
	case TypeMultiSelect:
		values, ok := toStrings(stored)
		if !ok {
			return false
		}
		for _, value := range values {
			if !hasOption(def, value) {
				return false
			}
		}
		return true
	}
	return false
}

// Display formats a stored value for timelines, emails and AI context. It
// returns "" for values that do not fit.
func Display(def Definition, stored any) string {
	if stored == nil || !Fits(def, stored) {
		return ""
	}
	switch def.Type {
	case TypeNumber:
		number, _ := toNumber(stored)
		return strconv.FormatFloat(number, 'f', -1, 64)
	case TypeBoolean:
		if stored.(bool) {
			return "Yes"
		}
		return "No"
	case TypeSelect:
		return optionLabel(def, stored.(string))
	case TypeMultiSelect:
		values, _ := toStrings(stored)
		labels := make([]string, 0, len(values))
		for _, value := range values {
			labels = append(labels, optionLabel(def, value))
		}
		return strings.Join(labels, ", ")
	}
	text, _ := stored.(string)
	return strings.TrimSpace(text)
}

func hasOption(def Definition, value string) bool {
	return slices.ContainsFunc(def.Options, func(option Option) bool { return option.Value == value })
}

func optionLabel(def Definition, value string) string {
	for _, option := range def.Options {
		if option.Value == value {
			return option.Label
		}
	}
	return value
}

// toNumber accepts JSON numbers and strings holding only a decimal number,
// as CRM number filters do.
func toNumber(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, !math.IsNaN(value) && !math.IsInf(value, 0)
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case string:
		text := strings.TrimSpace(value)
		if text == "" || strings.ContainsAny(text, "eE+xX_") {
			return 0, false
		}
		number, err := strconv.ParseFloat(text, 64)
		return number, err == nil && !math.IsNaN(number) && !math.IsInf(number, 0)
	}
	return 0, false
}

func toStrings(raw any) ([]string, bool) {
	switch value := raw.(type) {
	case []string:
		return value, true
	case []any:
		result := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, text)
		}
		return result, true
	}
	return nil, false
}
