//go:build ee

package pricing

import (
	"errors"
	"strings"
)

// ValidateFlatTokenTariff rejects incomplete or unsupported SaaS token tariffs.
func ValidateFlatTokenTariff(t *FlatTokenTariff) error {
	if t == nil || strings.TrimSpace(t.Version) == "" || t.Currency != "USD" ||
		t.MicrousdPerMillion == nil || *t.MicrousdPerMillion < 0 || t.AccountingVersion != FlatTokenAccountingVersion {
		return errors.New("BYOK requires a versioned USD flat token tariff with an explicit nonnegative rate")
	}
	return nil
}

// FlatTokenRates expands the flat tariff for the shared exact-integer calculator.
// Call ValidateFlatTokenTariff before using the resulting rates.
func FlatTokenRates(t FlatTokenTariff) TokenRates {
	var rate int64
	if t.MicrousdPerMillion != nil {
		rate = *t.MicrousdPerMillion
	}
	return TokenRates{InputMicrousdPerMillion: rate, OutputMicrousdPerMillion: rate,
		CacheReadMicrousdPerMillion: rate, CacheWriteMicrousdPerMillion: rate}
}
