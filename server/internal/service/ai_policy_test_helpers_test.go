package service

import "github.com/helpin-ai/helpin/server/internal/aiusage"

func billingStringPtr(v string) *string { return &v }

func testFlatTariff(rate int64) *aiusage.FlatTokenTariff {
	return &aiusage.FlatTokenTariff{Version: "byok-2026-09", Currency: "USD",
		MicrousdPerMillion: &rate, AccountingVersion: aiusage.FlatTokenAccountingVersion}
}
