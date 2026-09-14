package aiusage

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
