package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDigits = 6
	defaultPeriod = 30 * time.Second
)

var base32Encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret creates a 20-byte base32-encoded TOTP secret.
func GenerateSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return base32Encoding.EncodeToString(raw), nil
}

// BuildProvisioningURI creates an otpauth provisioning URI for authenticator apps.
func BuildProvisioningURI(secret, accountName, issuer string) string {
	trimmedSecret := strings.ToUpper(strings.TrimSpace(secret))
	trimmedAccount := strings.TrimSpace(accountName)
	trimmedIssuer := strings.TrimSpace(issuer)

	label := url.PathEscape(trimmedIssuer) + ":" + url.PathEscape(trimmedAccount)
	query := url.Values{}
	query.Set("secret", trimmedSecret)
	query.Set("issuer", trimmedIssuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", strconv.Itoa(defaultDigits))
	query.Set("period", strconv.Itoa(int(defaultPeriod.Seconds())))

	return "otpauth://totp/" + label + "?" + query.Encode()
}

// ValidateOTP validates a six-digit TOTP code using the provided time skew window.
func ValidateOTP(secret, code string, window int) bool {
	return validateOTPAt(secret, code, window, time.Now().UTC())
}

// GenerateCode returns the TOTP code for the provided secret at the current time.
func GenerateCode(secret string, at time.Time) (string, error) {
	return generateCodeAt(secret, at.UTC())
}

func validateOTPAt(secret, code string, window int, now time.Time) bool {
	normalizedSecret := strings.ToUpper(strings.TrimSpace(secret))
	normalizedCode := normalizeOTPCode(code)
	if normalizedSecret == "" || len(normalizedCode) != defaultDigits {
		return false
	}

	if window < 0 {
		window = 0
	}

	for offset := -window; offset <= window; offset++ {
		generated, err := generateCodeAt(normalizedSecret, now.Add(time.Duration(offset)*defaultPeriod))
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(generated), []byte(normalizedCode)) == 1 {
			return true
		}
	}

	return false
}

func generateCodeAt(secret string, at time.Time) (string, error) {
	key, err := base32Encoding.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("decode totp secret: %w", err)
	}

	counter := uint64(at.Unix() / int64(defaultPeriod.Seconds()))
	var counterBytes [8]byte
	binary.BigEndian.PutUint64(counterBytes[:], counter)

	hash := hmac.New(sha1.New, key)
	if _, err := hash.Write(counterBytes[:]); err != nil {
		return "", fmt.Errorf("hash totp counter: %w", err)
	}

	sum := hash.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset : offset+4])
	truncated &= 0x7fffffff

	modulus := uint32(1)
	for i := 0; i < defaultDigits; i++ {
		modulus *= 10
	}

	return fmt.Sprintf("%0*d", defaultDigits, truncated%modulus), nil
}

func normalizeOTPCode(code string) string {
	replacer := strings.NewReplacer(" ", "", "-", "")
	return replacer.Replace(strings.TrimSpace(code))
}
