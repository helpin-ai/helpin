package totp

import "testing"

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes(10)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes() error = %v", err)
	}
	if len(codes) != 10 {
		t.Fatalf("expected 10 codes, got %d", len(codes))
	}

	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if len(code) != 8 {
			t.Fatalf("expected 8-character code, got %q", code)
		}
		if _, exists := seen[code]; exists {
			t.Fatalf("expected unique recovery codes, duplicated %q", code)
		}
		seen[code] = struct{}{}
	}
}

func TestHashRecoveryCode(t *testing.T) {
	first := HashRecoveryCode("ABCD1234")
	second := HashRecoveryCode("abcd-1234")
	other := HashRecoveryCode("WXYZ9876")

	if first == "" {
		t.Fatal("expected non-empty recovery code hash")
	}
	if first != second {
		t.Fatal("expected normalization to make hashes match")
	}
	if first == other {
		t.Fatal("expected different codes to produce different hashes")
	}
}
