//go:build ee

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestExportMatchesCommittedPublicPricing(t *testing.T) {
	var output bytes.Buffer
	if err := run([]string{"export", "--format", "typescript"}, &output); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../../frontend/src/ee/generated/aiPricing.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatal("public pricing export differs; regenerate the committed artifact")
	}
}

func TestRunValidateReportsActivePricingVersion(t *testing.T) {
	var output bytes.Buffer

	if err := run([]string{"validate"}, &output); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := output.String(); got != "pricing 2026-09-14 valid\n" {
		t.Fatalf("run() output = %q", got)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var output bytes.Buffer

	err := run([]string{"publish"}, &output)
	if err == nil || !strings.Contains(err.Error(), "usage: ai-pricing validate") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunExportTypeScriptIsDeterministic(t *testing.T) {
	var first, second bytes.Buffer
	for _, output := range []*bytes.Buffer{&first, &second} {
		if err := run([]string{"export", "--format", "typescript"}, output); err != nil {
			t.Fatalf("run(export) error = %v", err)
		}
	}
	if first.String() != second.String() {
		t.Fatal("TypeScript export is not deterministic")
	}
	for _, expected := range []string{"export const AI_PRICING", `"pricing_version": "2026-09-14`, "catalogSha256"} {
		if !strings.Contains(first.String(), expected) {
			t.Fatalf("export missing %q: %s", expected, first.String())
		}
	}
}
