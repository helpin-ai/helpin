package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func generateKey(t *testing.T, size int) []byte {
	t.Helper()
	key := make([]byte, size)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := generateKey(t, 32)

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"short message", []byte("hello world")},
		{"unicode text", []byte("こんにちは世界 🌍")},
		{"binary data", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}},
		{"single byte", []byte("x")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := Encrypt(tt.plaintext, key)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			got, err := Decrypt(ciphertext, key)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if !bytes.Equal(got, tt.plaintext) {
				t.Errorf("Decrypt() = %v, want %v", got, tt.plaintext)
			}
		})
	}
}

func TestEncryptStringDecryptStringRoundtrip(t *testing.T) {
	key := generateKey(t, 32)

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple string", "hello world"},
		{"unicode", "日本語テスト"},
		{"with special chars", "p@$$w0rd!#%&*()"},
		{"json payload", `{"token":"abc123","scope":"read"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := EncryptString(tt.plaintext, key)
			if err != nil {
				t.Fatalf("EncryptString() error = %v", err)
			}

			// Verify the ciphertext is valid base64
			if _, err := base64.StdEncoding.DecodeString(ciphertext); err != nil {
				t.Fatalf("EncryptString() produced invalid base64: %v", err)
			}

			got, err := DecryptString(ciphertext, key)
			if err != nil {
				t.Fatalf("DecryptString() error = %v", err)
			}

			if got != tt.plaintext {
				t.Errorf("DecryptString() = %q, want %q", got, tt.plaintext)
			}
		})
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := generateKey(t, 32)
	key2 := generateKey(t, 32)

	plaintext := []byte("secret data")

	ciphertext, err := Encrypt(plaintext, key1)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	_, err = Decrypt(ciphertext, key2)
	if err == nil {
		t.Error("Decrypt() with wrong key should return error, got nil")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key := generateKey(t, 32)
	plaintext := []byte("important data that must not be tampered with")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Flip a byte in the middle of the ciphertext (past the nonce)
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	mid := len(tampered) / 2
	tampered[mid] ^= 0xFF

	_, err = Decrypt(tampered, key)
	if err == nil {
		t.Error("Decrypt() with tampered ciphertext should return error, got nil")
	}
}

func TestEncryptKeyTooShort(t *testing.T) {
	shortKey := generateKey(t, 16)
	plaintext := []byte("test")

	_, err := Encrypt(plaintext, shortKey)
	if err == nil {
		t.Error("Encrypt() with 16-byte key should return error, got nil")
	}

	_, err = Decrypt([]byte("dummy ciphertext data here!!"), shortKey)
	if err == nil {
		t.Error("Decrypt() with 16-byte key should return error, got nil")
	}
}

func TestEncryptKeyTooLong(t *testing.T) {
	longKey := generateKey(t, 64)
	plaintext := []byte("test")

	_, err := Encrypt(plaintext, longKey)
	if err == nil {
		t.Error("Encrypt() with 64-byte key should return error, got nil")
	}

	_, err = Decrypt([]byte("dummy ciphertext data here!!"), longKey)
	if err == nil {
		t.Error("Decrypt() with 64-byte key should return error, got nil")
	}
}

func TestEmptyPlaintext(t *testing.T) {
	key := generateKey(t, 32)

	t.Run("Encrypt/Decrypt", func(t *testing.T) {
		ciphertext, err := Encrypt([]byte{}, key)
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}

		got, err := Decrypt(ciphertext, key)
		if err != nil {
			t.Fatalf("Decrypt() error = %v", err)
		}

		if len(got) != 0 {
			t.Errorf("Decrypt() = %v, want empty slice", got)
		}
	})

	t.Run("EncryptString/DecryptString", func(t *testing.T) {
		ciphertext, err := EncryptString("", key)
		if err != nil {
			t.Fatalf("EncryptString() error = %v", err)
		}

		got, err := DecryptString(ciphertext, key)
		if err != nil {
			t.Fatalf("DecryptString() error = %v", err)
		}

		if got != "" {
			t.Errorf("DecryptString() = %q, want empty string", got)
		}
	})
}

func TestLargePlaintext(t *testing.T) {
	key := generateKey(t, 32)

	// 1 MB of random data
	plaintext := make([]byte, 1<<20)
	if _, err := rand.Read(plaintext); err != nil {
		t.Fatalf("generate plaintext: %v", err)
	}

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	got, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Error("Decrypt() result does not match original 1MB plaintext")
	}
}

func TestDecryptCiphertextTooShort(t *testing.T) {
	key := generateKey(t, 32)

	// AES-GCM nonce is 12 bytes; anything shorter should fail
	shortInputs := [][]byte{
		{},
		{0x01},
		{0x01, 0x02, 0x03, 0x04, 0x05},
		make([]byte, 11),
	}

	for _, input := range shortInputs {
		_, err := Decrypt(input, key)
		if err == nil {
			t.Errorf("Decrypt() with %d-byte ciphertext should return error, got nil", len(input))
		}
	}
}

func TestDecryptStringInvalidBase64(t *testing.T) {
	key := generateKey(t, 32)

	invalidInputs := []struct {
		name  string
		input string
	}{
		{"not base64", "this is not valid base64!!!"},
		{"truncated padding", "aGVsbG8==" },
		{"illegal characters", "abc~def@ghi"},
	}

	for _, tt := range invalidInputs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptString(tt.input, key)
			if err == nil {
				t.Errorf("DecryptString(%q) should return error, got nil", tt.input)
			}
		})
	}
}

func TestEncryptProducesDifferentCiphertextEachTime(t *testing.T) {
	key := generateKey(t, 32)
	plaintext := []byte("same plaintext every time")

	ciphertext1, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() first call error = %v", err)
	}

	ciphertext2, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() second call error = %v", err)
	}

	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Encrypt() produced identical ciphertext for two calls with the same plaintext; nonce should be random")
	}

	// Both should still decrypt to the same plaintext
	got1, err := Decrypt(ciphertext1, key)
	if err != nil {
		t.Fatalf("Decrypt(ciphertext1) error = %v", err)
	}
	got2, err := Decrypt(ciphertext2, key)
	if err != nil {
		t.Fatalf("Decrypt(ciphertext2) error = %v", err)
	}

	if !bytes.Equal(got1, plaintext) || !bytes.Equal(got2, plaintext) {
		t.Error("both ciphertexts should decrypt to the original plaintext")
	}
}
