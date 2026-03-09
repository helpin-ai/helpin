package worker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func toolReadFile(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	absPath, err := safePath(ctx.WorkDir, params.Path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	if isBinaryContent(data) {
		return "", fmt.Errorf("file appears to be binary, cannot read: %s", params.Path)
	}

	// Truncate very large files.
	content := string(data)
	if len(content) > 100_000 {
		content = content[:100_000] + "\n... (truncated)"
	}
	return content, nil
}

func toolWriteFile(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	absPath, err := safePath(ctx.WorkDir, params.Path)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("create directories: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("Wrote %d bytes to %s", len(params.Content), params.Path), nil
}

func toolListDirectory(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	dirPath := ctx.WorkDir
	if params.Path != "" {
		p, err := safePath(ctx.WorkDir, params.Path)
		if err != nil {
			return "", err
		}
		dirPath = p
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("list directory: %w", err)
	}

	var lines []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	return strings.Join(lines, "\n"), nil
}

func toolSearchFiles(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Pattern string `json:"pattern"`
		Query   string `json:"query"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	matches, err := filepath.Glob(filepath.Join(ctx.WorkDir, params.Pattern))
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}

	if params.Query == "" {
		var results []string
		for _, m := range matches {
			rel, _ := filepath.Rel(ctx.WorkDir, m)
			results = append(results, rel)
		}
		if len(results) > 200 {
			results = results[:200]
			results = append(results, "... (truncated)")
		}
		return strings.Join(results, "\n"), nil
	}

	// Grep within matched files.
	var results []string
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := os.ReadFile(m)
		if err != nil || isBinaryContent(data) {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, params.Query) {
				rel, _ := filepath.Rel(ctx.WorkDir, m)
				results = append(results, fmt.Sprintf("%s:%d: %s", rel, i+1, line))
				if len(results) >= 100 {
					results = append(results, "... (truncated)")
					return strings.Join(results, "\n"), nil
				}
			}
		}
	}

	if len(results) == 0 {
		return "No matches found.", nil
	}
	return strings.Join(results, "\n"), nil
}

// isBinaryContent returns true if data looks like a binary file.
func isBinaryContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// Use net/http's content type detection on the first 512 bytes.
	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	ct := http.DetectContentType(sniff)
	return !strings.HasPrefix(ct, "text/") && ct != "application/json" && ct != "application/xml"
}

// safePath resolves a relative path within workDir and ensures it doesn't escape.
func safePath(workDir, relPath string) (string, error) {
	absPath := filepath.Join(workDir, filepath.Clean(relPath))
	if !strings.HasPrefix(absPath, workDir) {
		return "", fmt.Errorf("path traversal not allowed: %s", relPath)
	}
	return absPath, nil
}
