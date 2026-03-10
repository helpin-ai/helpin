package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	t.Run("hash and check correct password", func(t *testing.T) {
		password := "correct-horse-battery-staple"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() returned unexpected error: %v", err)
		}
		if hash == "" {
			t.Fatal("HashPassword() returned empty hash")
		}
		if err := CheckPassword(password, hash); err != nil {
			t.Errorf("CheckPassword() with correct password returned error: %v", err)
		}
	})

	t.Run("check with wrong password returns error", func(t *testing.T) {
		password := "correct-password"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() returned unexpected error: %v", err)
		}
		if err := CheckPassword("wrong-password", hash); err == nil {
			t.Error("CheckPassword() with wrong password expected error, got nil")
		}
	})

	t.Run("check with empty password against valid hash returns error", func(t *testing.T) {
		hash, err := HashPassword("some-password")
		if err != nil {
			t.Fatalf("HashPassword() returned unexpected error: %v", err)
		}
		if err := CheckPassword("", hash); err == nil {
			t.Error("CheckPassword() with empty password expected error, got nil")
		}
	})

	t.Run("different passwords produce different hashes", func(t *testing.T) {
		hash1, err := HashPassword("password-one")
		if err != nil {
			t.Fatalf("HashPassword(password-one) returned unexpected error: %v", err)
		}
		hash2, err := HashPassword("password-two")
		if err != nil {
			t.Fatalf("HashPassword(password-two) returned unexpected error: %v", err)
		}
		if hash1 == hash2 {
			t.Error("HashPassword() produced identical hashes for different passwords")
		}
	})

	t.Run("same password twice produces different hashes due to salt", func(t *testing.T) {
		password := "same-password"
		hash1, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() first call returned unexpected error: %v", err)
		}
		hash2, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() second call returned unexpected error: %v", err)
		}
		if hash1 == hash2 {
			t.Error("HashPassword() produced identical hashes for same password; expected different salts")
		}
		// Both hashes should still verify against the original password
		if err := CheckPassword(password, hash1); err != nil {
			t.Errorf("CheckPassword() with hash1 returned error: %v", err)
		}
		if err := CheckPassword(password, hash2); err != nil {
			t.Errorf("CheckPassword() with hash2 returned error: %v", err)
		}
	})

	t.Run("check with empty hash returns error", func(t *testing.T) {
		if err := CheckPassword("some-password", ""); err == nil {
			t.Error("CheckPassword() with empty hash expected error, got nil")
		}
	})

	t.Run("check with invalid malformed hash returns error", func(t *testing.T) {
		malformedHashes := []string{
			"not-a-bcrypt-hash",
			"$2a$10$invalid",
			"$2a$10$toolong" + string(make([]byte, 100)),
			"123456",
		}
		for _, bad := range malformedHashes {
			if err := CheckPassword("any-password", bad); err == nil {
				t.Errorf("CheckPassword() with malformed hash %q expected error, got nil", bad)
			}
		}
	})

	t.Run("hash empty string succeeds", func(t *testing.T) {
		hash, err := HashPassword("")
		if err != nil {
			t.Fatalf("HashPassword(\"\") returned unexpected error: %v", err)
		}
		if hash == "" {
			t.Fatal("HashPassword(\"\") returned empty hash")
		}
		// The empty string hash should verify correctly
		if err := CheckPassword("", hash); err != nil {
			t.Errorf("CheckPassword(\"\", hash) returned error: %v", err)
		}
		// A non-empty password should not match the empty-string hash
		if err := CheckPassword("not-empty", hash); err == nil {
			t.Error("CheckPassword(\"not-empty\", emptyHash) expected error, got nil")
		}
	})
}

func TestHashPasswordUsesDefaultCost(t *testing.T) {
	hash, err := HashPassword("test-password")
	if err != nil {
		t.Fatalf("HashPassword() returned unexpected error: %v", err)
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost() returned unexpected error: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Errorf("expected bcrypt cost %d, got %d", bcrypt.DefaultCost, cost)
	}
}
