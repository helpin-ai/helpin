package service

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"reflect"
)

// JSONB normalizes object order and whitespace. Comparing serialized bytes
// makes every reconciliation look like a new message/interaction/usage update.
func agentRuntimeProjectionJSONRawEqual(left, right json.RawMessage) bool {
	left, right = bytes.TrimSpace(left), bytes.TrimSpace(right)
	if bytes.Equal(left, right) {
		return true
	}
	decode := func(raw json.RawMessage) (any, bool) {
		if len(raw) == 0 {
			return nil, true // nullable JSON columns and absent optional payloads
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber() // preserve integer identifiers above float64 precision
		var value any
		if decoder.Decode(&value) != nil {
			return nil, false
		}
		return value, decoder.Decode(new(any)) == io.EOF
	}
	l, leftOK := decode(left)
	r, rightOK := decode(right)
	return leftOK && rightOK && agentRuntimeJSONValueEqual(l, r)
}

func agentRuntimeJSONValueEqual(left, right any) bool {
	switch l := left.(type) {
	case map[string]any:
		r, ok := right.(map[string]any)
		if !ok || len(l) != len(r) {
			return false
		}
		for key, value := range l {
			other, exists := r[key]
			if !exists || !agentRuntimeJSONValueEqual(value, other) {
				return false
			}
		}
		return true
	case []any:
		r, ok := right.([]any)
		if !ok || len(l) != len(r) {
			return false
		}
		for i, value := range l {
			if !agentRuntimeJSONValueEqual(value, r[i]) {
				return false
			}
		}
		return true
	case json.Number:
		r, ok := right.(json.Number)
		if !ok {
			return false
		}
		if l == r {
			return true
		}
		ln, leftOK := new(big.Rat).SetString(string(l))
		rn, rightOK := new(big.Rat).SetString(string(r))
		return leftOK && rightOK && ln.Cmp(rn) == 0
	default:
		return reflect.DeepEqual(left, right)
	}
}
