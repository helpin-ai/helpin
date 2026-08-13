package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunValidateReportsActivePricingVersion(t *testing.T) {
	var output bytes.Buffer

	if err := run([]string{"validate"}, &output); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := output.String(); got != "pricing 2026-08-13 valid\n" {
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
