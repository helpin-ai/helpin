package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	patchOpAdd    = "add"
	patchOpUpdate = "update"
	patchOpDelete = "delete"
)

type applyPatchFileOp struct {
	OpType   string
	Path     string
	MoveTo   string
	AddLines []string
	Hunks    []applyPatchHunk
}

type applyPatchHunk struct {
	OldLines []string
	NewLines []string
}

type patchPlannedWrite struct {
	Path    string
	Content *string
	Mode    os.FileMode
}

func toolApplyPatch(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Patch string `json:"patch"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.Patch) == "" {
		return "", fmt.Errorf("patch is required")
	}

	ops, err := parseApplyPatch(params.Patch)
	if err != nil {
		return "", err
	}
	plannedWrites, err := buildPatchPlan(ctx, ops)
	if err != nil {
		return "", err
	}
	if err := applyPatchPlan(ctx, plannedWrites); err != nil {
		return "", err
	}

	changed := make([]string, 0, len(plannedWrites))
	for _, write := range plannedWrites {
		changed = append(changed, write.Path)
	}
	return fmt.Sprintf("Applied patch touching %d file(s): %s", len(changed), strings.Join(changed, ", ")), nil
}

func parseApplyPatch(text string) ([]applyPatchFileOp, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "*** Begin Patch" {
		return nil, fmt.Errorf("patch must start with *** Begin Patch")
	}

	var ops []applyPatchFileOp
	for i := 1; i < len(lines); {
		line := lines[i]
		switch {
		case line == "":
			i++
		case line == "*** End Patch":
			return ops, nil
		case strings.HasPrefix(line, "*** Add File: "):
			op, next, err := parseAddFileOp(lines, i)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
			i = next
		case strings.HasPrefix(line, "*** Delete File: "):
			op := applyPatchFileOp{
				OpType: patchOpDelete,
				Path:   strings.TrimSpace(strings.TrimPrefix(line, "*** Delete File: ")),
			}
			if op.Path == "" {
				return nil, fmt.Errorf("delete file path is required")
			}
			ops = append(ops, op)
			i++
		case strings.HasPrefix(line, "*** Update File: "):
			op, next, err := parseUpdateFileOp(lines, i)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
			i = next
		default:
			return nil, fmt.Errorf("unexpected patch line: %s", line)
		}
	}

	return nil, fmt.Errorf("patch is missing *** End Patch")
}

func parseAddFileOp(lines []string, start int) (applyPatchFileOp, int, error) {
	op := applyPatchFileOp{
		OpType: patchOpAdd,
		Path:   strings.TrimSpace(strings.TrimPrefix(lines[start], "*** Add File: ")),
	}
	if op.Path == "" {
		return applyPatchFileOp{}, 0, fmt.Errorf("add file path is required")
	}

	i := start + 1
	for i < len(lines) {
		line := lines[i]
		if isPatchSectionBoundary(line) {
			break
		}
		if line == "*** End of File" {
			i++
			continue
		}
		if !strings.HasPrefix(line, "+") {
			return applyPatchFileOp{}, 0, fmt.Errorf("add file content must use '+' lines for %s", op.Path)
		}
		op.AddLines = append(op.AddLines, line[1:])
		i++
	}

	return op, i, nil
}

func parseUpdateFileOp(lines []string, start int) (applyPatchFileOp, int, error) {
	op := applyPatchFileOp{
		OpType: patchOpUpdate,
		Path:   strings.TrimSpace(strings.TrimPrefix(lines[start], "*** Update File: ")),
	}
	if op.Path == "" {
		return applyPatchFileOp{}, 0, fmt.Errorf("update file path is required")
	}

	i := start + 1
	if i < len(lines) && strings.HasPrefix(lines[i], "*** Move to: ") {
		op.MoveTo = strings.TrimSpace(strings.TrimPrefix(lines[i], "*** Move to: "))
		if op.MoveTo == "" {
			return applyPatchFileOp{}, 0, fmt.Errorf("move destination is required for %s", op.Path)
		}
		i++
	}

	for i < len(lines) {
		line := lines[i]
		if isPatchSectionBoundary(line) {
			break
		}
		if !strings.HasPrefix(line, "@@") {
			return applyPatchFileOp{}, 0, fmt.Errorf("expected @@ hunk header for %s, got %s", op.Path, line)
		}

		hunk := applyPatchHunk{}
		i++
		for i < len(lines) {
			line = lines[i]
			if strings.HasPrefix(line, "@@") || isPatchSectionBoundary(line) {
				break
			}
			if line == "*** End of File" {
				i++
				continue
			}
			if line == "" {
				return applyPatchFileOp{}, 0, fmt.Errorf("unexpected blank line inside patch hunk for %s", op.Path)
			}
			switch line[0] {
			case ' ':
				hunk.OldLines = append(hunk.OldLines, line[1:])
				hunk.NewLines = append(hunk.NewLines, line[1:])
			case '-':
				hunk.OldLines = append(hunk.OldLines, line[1:])
			case '+':
				hunk.NewLines = append(hunk.NewLines, line[1:])
			default:
				return applyPatchFileOp{}, 0, fmt.Errorf("invalid patch line in %s: %s", op.Path, line)
			}
			i++
		}
		if len(hunk.OldLines) == 0 {
			return applyPatchFileOp{}, 0, fmt.Errorf("patch hunk for %s must include at least one context or removed line", op.Path)
		}
		op.Hunks = append(op.Hunks, hunk)
	}

	if len(op.Hunks) == 0 {
		return applyPatchFileOp{}, 0, fmt.Errorf("update file %s must include at least one hunk", op.Path)
	}
	return op, i, nil
}

func isPatchSectionBoundary(line string) bool {
	return line == "*** End Patch" ||
		strings.HasPrefix(line, "*** Add File: ") ||
		strings.HasPrefix(line, "*** Update File: ") ||
		strings.HasPrefix(line, "*** Delete File: ")
}

func buildPatchPlan(ctx *ExecutionContext, ops []applyPatchFileOp) ([]patchPlannedWrite, error) {
	if len(ops) == 0 {
		return nil, fmt.Errorf("patch does not contain any file operations")
	}

	seenTargets := make(map[string]string, len(ops))
	plannedWrites := make([]patchPlannedWrite, 0, len(ops)*2)

	for _, op := range ops {
		switch op.OpType {
		case patchOpAdd:
			targetPath, err := safePath(ctx.WorkDir, op.Path)
			if err != nil {
				return nil, err
			}
			if prev, ok := seenTargets[targetPath]; ok {
				return nil, fmt.Errorf("patch touches %s more than once (%s)", op.Path, prev)
			}
			if _, err := os.Stat(targetPath); err == nil {
				return nil, fmt.Errorf("cannot add %s because it already exists", op.Path)
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("stat added file: %w", err)
			}
			content := strings.Join(op.AddLines, "\n")
			plannedWrites = append(plannedWrites, patchPlannedWrite{
				Path:    targetPath,
				Content: &content,
				Mode:    0644,
			})
			seenTargets[targetPath] = op.OpType
		case patchOpDelete:
			sourcePath, err := safePath(ctx.WorkDir, op.Path)
			if err != nil {
				return nil, err
			}
			if prev, ok := seenTargets[sourcePath]; ok {
				return nil, fmt.Errorf("patch touches %s more than once (%s)", op.Path, prev)
			}
			if err := validateToolFileMutation(ctx, sourcePath); err != nil {
				return nil, err
			}
			if _, err := os.Stat(sourcePath); err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("cannot delete %s because it does not exist", op.Path)
				}
				return nil, fmt.Errorf("stat deleted file: %w", err)
			}
			plannedWrites = append(plannedWrites, patchPlannedWrite{
				Path:    sourcePath,
				Content: nil,
			})
			seenTargets[sourcePath] = op.OpType
		case patchOpUpdate:
			sourcePath, err := safePath(ctx.WorkDir, op.Path)
			if err != nil {
				return nil, err
			}
			if err := validateToolFileMutation(ctx, sourcePath); err != nil {
				return nil, err
			}
			info, err := os.Stat(sourcePath)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("cannot update %s because it does not exist", op.Path)
				}
				return nil, fmt.Errorf("stat updated file: %w", err)
			}
			data, err := os.ReadFile(sourcePath)
			if err != nil {
				return nil, fmt.Errorf("read file for patch: %w", err)
			}
			if isBinaryContent(data) {
				return nil, fmt.Errorf("file appears to be binary, cannot patch: %s", op.Path)
			}

			updatedContent, err := applyPatchHunks(string(data), op.Path, op.Hunks)
			if err != nil {
				return nil, err
			}

			targetPath := sourcePath
			if strings.TrimSpace(op.MoveTo) != "" {
				targetPath, err = safePath(ctx.WorkDir, op.MoveTo)
				if err != nil {
					return nil, err
				}
				if targetPath != sourcePath {
					if prev, ok := seenTargets[targetPath]; ok {
						return nil, fmt.Errorf("patch touches %s more than once (%s)", op.MoveTo, prev)
					}
					if _, err := os.Stat(targetPath); err == nil {
						return nil, fmt.Errorf("cannot move %s to %s because the destination already exists", op.Path, op.MoveTo)
					} else if !os.IsNotExist(err) {
						return nil, fmt.Errorf("stat move destination: %w", err)
					}
					plannedWrites = append(plannedWrites, patchPlannedWrite{
						Path:    sourcePath,
						Content: nil,
					})
					seenTargets[sourcePath] = patchOpDelete
				}
			}
			if prev, ok := seenTargets[targetPath]; ok {
				return nil, fmt.Errorf("patch touches %s more than once (%s)", relativeToolPath(ctx, targetPath), prev)
			}
			contentCopy := updatedContent
			plannedWrites = append(plannedWrites, patchPlannedWrite{
				Path:    targetPath,
				Content: &contentCopy,
				Mode:    info.Mode().Perm(),
			})
			seenTargets[targetPath] = op.OpType
		default:
			return nil, fmt.Errorf("unsupported patch operation %q", op.OpType)
		}
	}

	return plannedWrites, nil
}

func applyPatchHunks(content, relPath string, hunks []applyPatchHunk) (string, error) {
	lines := strings.Split(content, "\n")
	for _, hunk := range hunks {
		start, err := findUniqueLineSequence(lines, hunk.OldLines)
		if err != nil {
			return "", fmt.Errorf("apply hunk for %s: %w", relPath, err)
		}
		updated := make([]string, 0, len(lines)-len(hunk.OldLines)+len(hunk.NewLines))
		updated = append(updated, lines[:start]...)
		updated = append(updated, hunk.NewLines...)
		updated = append(updated, lines[start+len(hunk.OldLines):]...)
		lines = updated
	}
	return strings.Join(lines, "\n"), nil
}

func findUniqueLineSequence(lines, needle []string) (int, error) {
	if len(needle) == 0 {
		return 0, fmt.Errorf("patch hunk has no matchable context")
	}
	matchIndex := -1
	for start := 0; start+len(needle) <= len(lines); start++ {
		if !equalStringSlices(lines[start:start+len(needle)], needle) {
			continue
		}
		if matchIndex != -1 {
			return 0, fmt.Errorf("hunk matched multiple locations; include more exact surrounding context")
		}
		matchIndex = start
	}
	if matchIndex == -1 {
		return 0, fmt.Errorf("hunk did not match the current file content; re-read the file and refresh the patch context")
	}
	return matchIndex, nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func applyPatchPlan(ctx *ExecutionContext, writes []patchPlannedWrite) error {
	for _, write := range writes {
		if write.Content == nil {
			if err := os.Remove(write.Path); err != nil {
				return fmt.Errorf("delete %s: %w", relativeToolPath(ctx, write.Path), err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(write.Path), 0755); err != nil {
			return fmt.Errorf("create directories for %s: %w", relativeToolPath(ctx, write.Path), err)
		}
		if err := os.WriteFile(write.Path, []byte(*write.Content), write.Mode); err != nil {
			return fmt.Errorf("write %s: %w", relativeToolPath(ctx, write.Path), err)
		}
		if info, statErr := os.Stat(write.Path); statErr == nil {
			recordToolFileWrite(ctx, write.Path, info.ModTime())
		}
	}
	return nil
}
