package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ToolFileObservation struct {
	LastReadAt      time.Time
	LastReadModTime time.Time
	ReadVia         string
}

type ToolFileState struct {
	mu    sync.Mutex
	reads map[string]ToolFileObservation
}

func NewToolFileState() *ToolFileState {
	return &ToolFileState{
		reads: make(map[string]ToolFileObservation),
	}
}

func ensureToolFileState(ctx *ExecutionContext) *ToolFileState {
	if ctx == nil {
		return nil
	}
	if ctx.ToolFileState == nil {
		ctx.ToolFileState = NewToolFileState()
	}
	return ctx.ToolFileState
}

func recordToolFileRead(ctx *ExecutionContext, absPath string, modTime time.Time, via string) {
	state := ensureToolFileState(ctx)
	if state == nil {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	state.reads[absPath] = ToolFileObservation{
		LastReadAt:      time.Now().UTC(),
		LastReadModTime: modTime.UTC(),
		ReadVia:         via,
	}
}

func recordToolFileWrite(ctx *ExecutionContext, absPath string, modTime time.Time) {
	state := ensureToolFileState(ctx)
	if state == nil {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	obs := state.reads[absPath]
	obs.LastReadAt = time.Now().UTC()
	obs.LastReadModTime = modTime.UTC()
	if obs.ReadVia == "" {
		obs.ReadVia = "write_file"
	}
	state.reads[absPath] = obs
}

func validateToolFileMutation(ctx *ExecutionContext, absPath string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat file before edit: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", relativeToolPath(ctx, absPath))
	}

	state := ensureToolFileState(ctx)
	if state == nil {
		return nil
	}

	state.mu.Lock()
	obs, ok := state.reads[absPath]
	state.mu.Unlock()
	if !ok {
		return fmt.Errorf("must read %s before modifying it; use read_file or read_file_range first", relativeToolPath(ctx, absPath))
	}

	currentModTime := info.ModTime().UTC()
	if currentModTime.After(obs.LastReadModTime) {
		return fmt.Errorf(
			"refusing to modify %s because it changed since the last %s call (last seen mod time %s, current mod time %s); re-read the file and try again",
			relativeToolPath(ctx, absPath),
			obs.ReadVia,
			obs.LastReadModTime.Format(time.RFC3339Nano),
			currentModTime.Format(time.RFC3339Nano),
		)
	}

	return nil
}

func relativeToolPath(ctx *ExecutionContext, absPath string) string {
	if ctx == nil || ctx.WorkDir == "" {
		return absPath
	}
	rel, err := filepath.Rel(ctx.WorkDir, absPath)
	if err != nil {
		return absPath
	}
	return rel
}
