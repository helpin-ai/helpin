package tiptap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// DocumentVersion identifies a stored content snapshot independently of JSON formatting.
func DocumentVersion(raw json.RawMessage) string {
	var value any
	if json.Unmarshal(raw, &value) == nil {
		if canonical, err := json.Marshal(value); err == nil {
			raw = canonical
		}
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
