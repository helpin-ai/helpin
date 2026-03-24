package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultReadFileLimitLines = 200
	maxReadFileLimitLines     = 400
)

func toolReadFile(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Path       string `json:"path"`
		OffsetLine int    `json:"offset_line"`
		LimitLines int    `json:"limit_lines"`
		Offset     int    `json:"offset"`
		Limit      int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	startLine, limitLines, err := normalizeReadFileWindow(params.OffsetLine, params.LimitLines, params.Offset, params.Limit)
	if err != nil {
		return "", err
	}

	absPath, err := safePath(ctx.WorkDir, params.Path)
	if err != nil {
		return "", err
	}

	f, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	preview := make([]byte, 512)
	n, readErr := f.Read(preview)
	if readErr != nil && readErr != io.EOF {
		return "", fmt.Errorf("read file preview: %w", readErr)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", fmt.Errorf("reset file cursor: %w", err)
	}

	if isBinaryContent(preview[:n]) {
		return "", fmt.Errorf("file appears to be binary, cannot read: %s", params.Path)
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	currentLine := 0
	collected := make([]string, 0, limitLines)
	for currentLine < startLine-1 && scanner.Scan() {
		currentLine++
	}

	for scanner.Scan() && len(collected) < limitLines {
		currentLine++
		collected = append(collected, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	if info, statErr := os.Stat(absPath); statErr == nil {
		recordToolFileRead(ctx, absPath, info.ModTime(), "read_file")
	}

	if len(collected) == 0 {
		return fmt.Sprintf("No lines available starting at line %d in %s", startLine, params.Path), nil
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("<file path=\"%s\" start_line=\"%d\" returned_lines=\"%d\">\n", params.Path, startLine, len(collected)))
	out.WriteString(strings.Join(collected, "\n"))
	out.WriteString("\n</file>")
	if scanner.Scan() {
		nextLine := startLine + len(collected)
		out.WriteString(fmt.Sprintf("\n\nFile has more lines. Use read_file with {\"path\":\"%s\",\"offset_line\":%d} to continue, or use read_file_range for a specific span.", params.Path, nextLine))
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return out.String(), nil
}

func normalizeReadFileWindow(offsetLine, limitLines, offset, limit int) (int, int, error) {
	startLine := 1
	if offsetLine > 0 {
		startLine = offsetLine
	} else if offset > 0 {
		startLine = offset + 1
	} else if offset < 0 {
		return 0, 0, fmt.Errorf("offset must be >= 0")
	}
	if startLine < 1 {
		return 0, 0, fmt.Errorf("offset_line must be >= 1")
	}

	if limitLines <= 0 {
		limitLines = limit
	}
	if limitLines <= 0 {
		limitLines = defaultReadFileLimitLines
	}
	if limitLines > maxReadFileLimitLines {
		return 0, 0, fmt.Errorf("limit_lines too large: max %d lines per call (requested %d)", maxReadFileLimitLines, limitLines)
	}
	return startLine, limitLines, nil
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

	var mode os.FileMode = 0644
	if info, statErr := os.Stat(absPath); statErr == nil {
		if info.IsDir() {
			return "", fmt.Errorf("path is a directory, not a file: %s", params.Path)
		}
		if err := validateToolFileMutation(ctx, absPath); err != nil {
			return "", err
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("stat file before write: %w", statErr)
	}

	if err := os.WriteFile(absPath, []byte(params.Content), mode); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	if info, statErr := os.Stat(absPath); statErr == nil {
		recordToolFileWrite(ctx, absPath, info.ModTime())
	}

	return fmt.Sprintf("Wrote %d bytes to %s", len(params.Content), params.Path), nil
}

func toolEditFile(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.Path) == "" {
		return "", fmt.Errorf("path is required")
	}
	if params.OldString == "" {
		return "", fmt.Errorf("old_string is required and must not be empty")
	}

	absPath, err := safePath(ctx.WorkDir, params.Path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file does not exist: %s", params.Path)
		}
		return "", fmt.Errorf("stat file before edit: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", params.Path)
	}
	if err := validateToolFileMutation(ctx, absPath); err != nil {
		return "", err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read file for edit: %w", err)
	}
	if isBinaryContent(data) {
		return "", fmt.Errorf("file appears to be binary, cannot edit: %s", params.Path)
	}

	content := string(data)
	matchCount := strings.Count(content, params.OldString)
	switch {
	case matchCount == 0:
		return "", fmt.Errorf("old_string did not match any content in %s; re-read the file and include more exact surrounding context", params.Path)
	case matchCount > 1:
		return "", fmt.Errorf("old_string matched %d locations in %s; include more surrounding context so the match is unique", matchCount, params.Path)
	}

	updated := strings.Replace(content, params.OldString, params.NewString, 1)
	if updated == content {
		return fmt.Sprintf("No changes made to %s.", params.Path), nil
	}

	if err := os.WriteFile(absPath, []byte(updated), info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("write edited file: %w", err)
	}
	if updatedInfo, statErr := os.Stat(absPath); statErr == nil {
		recordToolFileWrite(ctx, absPath, updatedInfo.ModTime())
	}

	return fmt.Sprintf("Edited %s by replacing 1 occurrence.", params.Path), nil
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

// ---------------------------------------------------------------------------
// read_file_range
// ---------------------------------------------------------------------------

func toolReadFileRange(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.StartLine < 1 || params.EndLine < 1 {
		return "", fmt.Errorf("start_line and end_line must be >= 1")
	}
	if params.EndLine < params.StartLine {
		return "", fmt.Errorf("end_line must be >= start_line")
	}
	if params.EndLine-params.StartLine+1 > 500 {
		return "", fmt.Errorf("range too large: max 500 lines per call (requested %d)", params.EndLine-params.StartLine+1)
	}

	absPath, err := safePath(ctx.WorkDir, params.Path)
	if err != nil {
		return "", err
	}

	f, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var lines []string
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum > params.EndLine {
			break
		}
		if lineNum >= params.StartLine {
			lines = append(lines, fmt.Sprintf("%4d | %s", lineNum, scanner.Text()))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	if info, statErr := os.Stat(absPath); statErr == nil {
		recordToolFileRead(ctx, absPath, info.ModTime(), "read_file_range")
	}

	if len(lines) == 0 {
		return fmt.Sprintf("No lines in range %d-%d (file has %d lines)", params.StartLine, params.EndLine, lineNum), nil
	}
	return strings.Join(lines, "\n"), nil
}

// ---------------------------------------------------------------------------
// ripgrep
// ---------------------------------------------------------------------------

func toolRipgrep(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Pattern         string `json:"pattern"`
		Path            string `json:"path"`
		FileType        string `json:"file_type"`
		ContextLines    int    `json:"context_lines"`
		MaxResults      int    `json:"max_results"`
		CaseInsensitive bool   `json:"case_insensitive"`
		FixedStrings    bool   `json:"fixed_strings"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// Defaults and caps.
	if params.MaxResults <= 0 {
		params.MaxResults = 50
	}
	if params.MaxResults > 200 {
		params.MaxResults = 200
	}
	if params.ContextLines < 0 {
		params.ContextLines = 0
	}
	if params.ContextLines > 5 {
		params.ContextLines = 5
	}

	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return "", fmt.Errorf("ripgrep (rg) not found in PATH; use the 'grep' tool as a fallback")
	}

	searchDir := ctx.WorkDir
	if params.Path != "" {
		searchDir, err = safePath(ctx.WorkDir, params.Path)
		if err != nil {
			return "", err
		}
	}

	args := []string{
		"--no-heading",
		"--line-number",
		"--color", "never",
		"--max-columns", "500",
		"--max-columns-preview",
		"--glob", "!.git",
		"--glob", "!node_modules",
		"--glob", "!vendor",
		"--glob", "!dist",
		"--glob", "!__pycache__",
	}

	if params.FileType != "" {
		args = append(args, "--type", params.FileType)
	}
	if params.ContextLines > 0 {
		args = append(args, "-C", strconv.Itoa(params.ContextLines))
	}
	if params.CaseInsensitive {
		args = append(args, "-i")
	}
	if params.FixedStrings {
		args = append(args, "-F")
	}

	args = append(args, "--", params.Pattern, searchDir)

	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeout, rgPath, args...)
	out, err := cmd.Output()

	// rg exits 1 when no matches found — that's not an error.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "No matches found.", nil
		}
		if timeout.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("ripgrep timed out after 30s")
		}
		return "", fmt.Errorf("ripgrep error: %w", err)
	}

	// Make paths relative to workdir.
	result := string(out)
	result = strings.ReplaceAll(result, ctx.WorkDir+"/", "")

	// Cap output lines.
	outputLines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(outputLines) > params.MaxResults {
		outputLines = outputLines[:params.MaxResults]
		outputLines = append(outputLines, fmt.Sprintf("... (%d+ results, truncated)", params.MaxResults))
	}

	return strings.Join(outputLines, "\n"), nil
}

// ---------------------------------------------------------------------------
// grep (Go-native fallback)
// ---------------------------------------------------------------------------

var excludedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"__pycache__":  true,
}

func toolGrep(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Pattern    string `json:"pattern"`
		Path       string `json:"path"`
		Include    string `json:"include"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if params.MaxResults <= 0 {
		params.MaxResults = 50
	}

	re, err := regexp.Compile(params.Pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	searchDir := ctx.WorkDir
	if params.Path != "" {
		searchDir, err = safePath(ctx.WorkDir, params.Path)
		if err != nil {
			return "", err
		}
	}

	var results []string
	walkErr := filepath.WalkDir(searchDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if excludedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if params.Include != "" {
			matched, _ := filepath.Match(params.Include, d.Name())
			if !matched {
				return nil
			}
		}

		// Skip large and binary files.
		info, err := d.Info()
		if err != nil || info.Size() > 2*1024*1024 {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil || isBinaryContent(data) {
			return nil
		}

		rel, _ := filepath.Rel(ctx.WorkDir, path)
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d: %s", rel, i+1, line))
				if len(results) >= params.MaxResults {
					return fmt.Errorf("limit reached")
				}
			}
		}
		return nil
	})

	if walkErr != nil && walkErr.Error() != "limit reached" {
		return "", fmt.Errorf("search error: %w", walkErr)
	}

	if len(results) == 0 {
		return "No matches found.", nil
	}
	if len(results) >= params.MaxResults {
		results = append(results, fmt.Sprintf("... (truncated at %d results)", params.MaxResults))
	}
	return strings.Join(results, "\n"), nil
}

// ---------------------------------------------------------------------------
// list_symbols
// ---------------------------------------------------------------------------

type symbolPattern struct {
	exts     []string
	patterns []*regexp.Regexp
}

var symbolPatterns = []symbolPattern{
	{
		exts: []string{".go"},
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`^func\s`),
			regexp.MustCompile(`^type\s+\w+\s+(struct|interface)`),
			regexp.MustCompile(`^type\s+\w+\s`),
			regexp.MustCompile(`^var\s+\w+`),
			regexp.MustCompile(`^const\s+\w+`),
		},
	},
	{
		exts: []string{".ts", ".tsx"},
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`^export\s+(function|const|class|type|interface|enum)\s`),
			regexp.MustCompile(`^\s*(public|private|protected|async)\s+\w+\(`),
			regexp.MustCompile(`^function\s+\w+`),
		},
	},
	{
		exts: []string{".js", ".jsx"},
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`^export\s+(function|const|class)\s`),
			regexp.MustCompile(`^function\s+\w+`),
			regexp.MustCompile(`^class\s+\w+`),
			regexp.MustCompile(`module\.exports`),
		},
	},
	{
		exts: []string{".py"},
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`^def\s+\w+`),
			regexp.MustCompile(`^class\s+\w+`),
			regexp.MustCompile(`^async\s+def\s+\w+`),
		},
	},
	{
		exts: []string{".rs"},
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`^pub\s+(fn|struct|enum|trait|type|impl|mod)\s`),
			regexp.MustCompile(`^fn\s+`),
			regexp.MustCompile(`^struct\s+`),
			regexp.MustCompile(`^enum\s+`),
			regexp.MustCompile(`^trait\s+`),
			regexp.MustCompile(`^impl\s`),
		},
	},
	{
		exts: []string{".java"},
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`(public|private|protected).*\s+(class|interface|enum)\s+`),
			regexp.MustCompile(`(public|private|protected)\s+.*\w+\s*\([^)]*\)\s*\{`),
		},
	},
}

func toolListSymbols(ctx *ExecutionContext, input json.RawMessage) (string, error) {
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

	ext := strings.ToLower(filepath.Ext(absPath))
	var patterns []*regexp.Regexp
	for _, sp := range symbolPatterns {
		for _, e := range sp.exts {
			if e == ext {
				patterns = sp.patterns
				break
			}
		}
		if patterns != nil {
			break
		}
	}
	if patterns == nil {
		return "", fmt.Errorf("unsupported file type: %s (supported: .go, .ts, .tsx, .js, .jsx, .py, .rs, .java)", ext)
	}

	f, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var symbols []string
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, p := range patterns {
			if p.MatchString(line) {
				symbols = append(symbols, fmt.Sprintf("%4d | %s", lineNum, line))
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	if len(symbols) == 0 {
		return "No symbols found.", nil
	}
	return strings.Join(symbols, "\n"), nil
}
