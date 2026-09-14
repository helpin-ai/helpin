package aiusage

import (
	"errors"
	"strings"
)

// FlatTokenAccountingVersion identifies the mutually exclusive token classes
// returned by NormalizeTokens. A rate applies equally to every class.
const FlatTokenAccountingVersion = "normalized-tokens-v1"

// FlatTokenTariff is an immutable, explicitly configured SaaS BYOK tariff.
// A nil rate is unconfigured; a non-nil zero rate is deliberately free.
type FlatTokenTariff struct {
	Version            string `json:"version"`
	Currency           string `json:"currency"`
	MicrousdPerMillion *int64 `json:"microusd_per_million"`
	AccountingVersion  string `json:"accounting_version"`
}

func (t *FlatTokenTariff) Validate() error {
	if t == nil || strings.TrimSpace(t.Version) == "" || t.Currency != "USD" ||
		t.MicrousdPerMillion == nil || *t.MicrousdPerMillion < 0 || t.AccountingVersion != FlatTokenAccountingVersion {
		return errors.New("BYOK requires a versioned USD flat token tariff with an explicit nonnegative rate")
	}
	return nil
}

// Rates expands the flat tariff for the shared exact-integer calculator.
// Call Validate before using the resulting rates.
func (t FlatTokenTariff) Rates() TokenRates {
	var rate int64
	if t.MicrousdPerMillion != nil {
		rate = *t.MicrousdPerMillion
	}
	return TokenRates{InputMicrousdPerMillion: rate, OutputMicrousdPerMillion: rate,
		CacheReadMicrousdPerMillion: rate, CacheWriteMicrousdPerMillion: rate}
}
