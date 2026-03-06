package githubapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
)

func TestParsePrivateKeySupportsRawAndBase64PEM(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	rawPEMBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	rawPEM := string(rawPEMBytes)
	base64PEM := base64.StdEncoding.EncodeToString(rawPEMBytes)

	cases := []struct {
		name  string
		value string
	}{
		{name: "raw pem", value: rawPEM},
		{name: "escaped newlines", value: strings.ReplaceAll(strings.TrimSpace(rawPEM), "\n", `\n`)},
		{name: "base64 pem", value: base64PEM},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := parsePrivateKey(tc.value)
			if err != nil {
				t.Fatalf("parse private key: %v", err)
			}
			if parsed.N.Cmp(key.N) != 0 {
				t.Fatalf("parsed key does not match original key")
			}
		})
	}
}
