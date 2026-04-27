package totp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const recoveryCodeCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateRecoveryCodes creates unique single-use recovery codes.
func GenerateRecoveryCodes(count int) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf("recovery code count must be positive")
	}

	codes := make([]string, 0, count)
	seen := make(map[string]struct{}, count)
	for len(codes) < count {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, err
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}

	return codes, nil
}

func generateRecoveryCode() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate recovery code: %w", err)
	}

	out := make([]byte, 8)
	for i, value := range raw {
		out[i] = recoveryCodeCharset[int(value)%len(recoveryCodeCharset)]
	}

	return string(out), nil
}

// HashRecoveryCode returns a stable SHA-256 hash of a normalized recovery code.
func HashRecoveryCode(code string) string {
	normalized := normalizeRecoveryCode(code)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func normalizeRecoveryCode(code string) string {
	replacer := strings.NewReplacer(" ", "", "-", "")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(code)))
}
