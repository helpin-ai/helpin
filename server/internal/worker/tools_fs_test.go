package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestToolWriteFileRequiresPriorReadForExistingFile(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "existing.txt")
	if err := os.WriteFile(filePath, []byte("original"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	_, err := toolWriteFile(ctx, json.RawMessage(`{"path":"existing.txt","content":"updated"}`))
	if err == nil || !strings.Contains(err.Error(), "must read existing.txt before modifying it") {
		t.Fatalf("expected prior-read error, got %v", err)
	}
}

func TestToolWriteFileRejectsStaleExistingFile(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "existing.txt")
	if err := os.WriteFile(filePath, []byte("original"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	if _, err := toolReadFile(ctx, json.RawMessage(`{"path":"existing.txt"}`)); err != nil {
		t.Fatalf("toolReadFile returned error: %v", err)
	}

	staleModTime := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(filePath, []byte("changed elsewhere"), 0644); err != nil {
		t.Fatalf("rewrite fixture: %v", err)
	}
	if err := os.Chtimes(filePath, staleModTime, staleModTime); err != nil {
		t.Fatalf("set stale mod time: %v", err)
	}

	_, err := toolWriteFile(ctx, json.RawMessage(`{"path":"existing.txt","content":"updated"}`))
	if err == nil || !strings.Contains(err.Error(), "changed since the last read_file call") {
		t.Fatalf("expected stale-read error, got %v", err)
	}
}

func TestToolWriteFileAllowsCreateWithoutPriorRead(t *testing.T) {
	workDir := t.TempDir()
	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	output, err := toolWriteFile(ctx, json.RawMessage(`{"path":"new.txt","content":"created"}`))
	if err != nil {
		t.Fatalf("toolWriteFile returned error: %v", err)
	}
	if !strings.Contains(output, "Wrote 7 bytes to new.txt") {
		t.Fatalf("unexpected output: %s", output)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "new.txt"))
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if string(data) != "created" {
		t.Fatalf("unexpected created file content %q", string(data))
	}
}

func TestToolEditFileRequiresUniqueMatch(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("alpha\nshared\nomega\nshared\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	if _, err := toolReadFile(ctx, json.RawMessage(`{"path":"sample.txt"}`)); err != nil {
		t.Fatalf("toolReadFile returned error: %v", err)
	}

	_, err := toolEditFile(ctx, json.RawMessage(`{"path":"sample.txt","old_string":"shared","new_string":"updated"}`))
	if err == nil || !strings.Contains(err.Error(), "matched 2 locations") {
		t.Fatalf("expected non-unique match error, got %v", err)
	}
}

func TestToolEditFileAppliesSingleReplacementAfterRead(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "sample.txt")
	initial := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(filePath, []byte(initial), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	if _, err := toolReadFile(ctx, json.RawMessage(`{"path":"sample.txt"}`)); err != nil {
		t.Fatalf("toolReadFile returned error: %v", err)
	}

	output, err := toolEditFile(ctx, json.RawMessage(`{"path":"sample.txt","old_string":"alpha\nbeta\ngamma\n","new_string":"alpha\nbeta-updated\ngamma\n"}`))
	if err != nil {
		t.Fatalf("toolEditFile returned error: %v", err)
	}
	if !strings.Contains(output, "Edited sample.txt by replacing 1 occurrence.") {
		t.Fatalf("unexpected output: %s", output)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read edited file: %v", err)
	}
	if string(data) != "alpha\nbeta-updated\ngamma\n" {
		t.Fatalf("unexpected edited content %q", string(data))
	}
}

func TestToolEditFileRequiresPriorRead(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	_, err := toolEditFile(ctx, json.RawMessage(`{"path":"sample.txt","old_string":"hello","new_string":"goodbye"}`))
	if err == nil || !strings.Contains(err.Error(), "must read sample.txt before modifying it") {
		t.Fatalf("expected prior-read error, got %v", err)
	}
}

func TestToolReadFileDefaultsToBoundedWindow(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "large.txt")
	var builder strings.Builder
	for i := 1; i <= defaultReadFileLimitLines+10; i++ {
		builder.WriteString("line ")
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString("\n")
	}
	content := builder.String()
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	output, err := toolReadFile(ctx, json.RawMessage(`{"path":"large.txt"}`))
	if err != nil {
		t.Fatalf("toolReadFile returned error: %v", err)
	}
	if !strings.Contains(output, `<file path="large.txt" start_line="1" returned_lines="200">`) {
		t.Fatalf("expected bounded read metadata, got %q", output)
	}
	if !strings.Contains(output, "line 1") || !strings.Contains(output, "line 200") {
		t.Fatalf("expected first window content, got %q", output)
	}
	if strings.Contains(output, "line 205") {
		t.Fatalf("did not expect lines past the default window, got %q", output)
	}
	if !strings.Contains(output, `"offset_line":201`) {
		t.Fatalf("expected continuation hint, got %q", output)
	}
}

func TestToolReadFileSupportsOffsetAndLimit(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("a\nb\nc\nd\ne\nf\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	output, err := toolReadFile(ctx, json.RawMessage(`{"path":"sample.txt","offset_line":3,"limit_lines":2}`))
	if err != nil {
		t.Fatalf("toolReadFile returned error: %v", err)
	}
	if !strings.Contains(output, `<file path="sample.txt" start_line="3" returned_lines="2">`) {
		t.Fatalf("expected offset metadata, got %q", output)
	}
	if !strings.Contains(output, "\nc\nd\n") {
		t.Fatalf("expected requested window content, got %q", output)
	}
	if strings.Contains(output, "\ne\n") {
		t.Fatalf("did not expect lines past requested limit, got %q", output)
	}
}

func TestToolReadFileRejectsOversizedLimit(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("one\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	_, err := toolReadFile(ctx, json.RawMessage(`{"path":"sample.txt","limit_lines":401}`))
	if err == nil || !strings.Contains(err.Error(), "limit_lines too large") {
		t.Fatalf("expected limit error, got %v", err)
	}
}
