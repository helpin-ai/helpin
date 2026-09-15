package main

import (
	"bytes"
	"os"
	"testing"
)

func TestExportMatchesCommittedModelCatalog(t *testing.T) {
	var output bytes.Buffer
	if err := export(&output); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../../frontend/src/generated/aiModels.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatal("model catalog drift: regenerate frontend/src/generated/aiModels.ts with cmd/ai-models")
	}
}
