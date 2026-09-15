package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/widgetorigin"
)

const (
	widgetIdentityVerifierVersion = "v1"
	widgetIdentityMaxValidity     = 15 * time.Minute
	widgetIdentityClockSkew       = time.Minute
)

type widgetIdentityProvenance struct {
	method          string
	trust           string
	verifiedAt      *time.Time
	verifierVersion *string
}

func verifyWidgetIdentity(
	installation *model.SupportWidgetInstallation,
	identity model.WidgetIdentityPayload,
	now time.Time,
) (widgetIdentityProvenance, error) {
	untrusted := widgetIdentityProvenance{
		method: model.IdentityMethodBrowserClaim,
		trust:  model.IdentityTrustUntrusted,
	}
	mode := strings.TrimSpace(installation.IdentityVerificationMode)
	if mode == "" {
		mode = model.IdentityVerificationModeReportOnly
	}
	if mode == model.IdentityVerificationModeOff {
		return untrusted, nil
	}

	verification := identity.IdentityVerification
	err := validateWidgetIdentityProof(installation, identity, verification, now)
	if err != nil {
		if mode == model.IdentityVerificationModeEnforced {
			return untrusted, fmt.Errorf("verified widget identity is required")
		}
		return untrusted, nil
	}

	verifiedAt := now.UTC()
	version := widgetIdentityVerifierVersion
	return widgetIdentityProvenance{
		method:          model.IdentityMethodSignedWidget,
		trust:           model.IdentityTrustVerified,
		verifiedAt:      &verifiedAt,
		verifierVersion: &version,
	}, nil
}

func validateWidgetIdentityProof(
	installation *model.SupportWidgetInstallation,
	identity model.WidgetIdentityPayload,
	verification *model.WidgetIdentityVerification,
	now time.Time,
) error {
	if verification == nil || verification.Version != widgetIdentityVerifierVersion {
		return fmt.Errorf("identity proof is missing or unsupported")
	}
	issuedAt := time.Unix(verification.IssuedAt, 0)
	expiresAt := time.Unix(verification.ExpiresAt, 0)
	if expiresAt.Before(now) || issuedAt.After(now.Add(widgetIdentityClockSkew)) {
		return fmt.Errorf("identity proof is outside its validity period")
	}
	if !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > widgetIdentityMaxValidity {
		return fmt.Errorf("identity proof validity exceeds maximum")
	}

	provided, err := hex.DecodeString(verification.Signature)
	if err != nil || len(provided) != sha256.Size {
		return fmt.Errorf("identity proof signature is invalid")
	}
	companyID := ""
	if identity.Company != nil {
		companyID = strings.TrimSpace(widgetIdentityValue(identity.Company["id"]))
	}
	message := strings.Join([]string{
		"helpin-widget-identity:v1",
		installation.WidgetKey,
		strings.ToLower(strings.TrimSpace(identity.Email)),
		identity.ExternalUserID,
		companyID,
		fmt.Sprintf("%d", verification.IssuedAt),
		fmt.Sprintf("%d", verification.ExpiresAt),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(installation.SecretKey))
	if _, err := mac.Write([]byte(message)); err != nil {
		return fmt.Errorf("sign identity proof: %w", err)
	}
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return fmt.Errorf("identity proof signature does not match")
	}
	return nil
}

func widgetIdentityValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func normalizeAllowedOrigins(origins []string) ([]string, error) {
	normalized := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))
	for _, raw := range origins {
		origin, err := widgetorigin.Normalize(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("invalid allowed origin: %w", err)
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		normalized = append(normalized, origin)
	}
	sort.Strings(normalized)
	return normalized, nil
}
