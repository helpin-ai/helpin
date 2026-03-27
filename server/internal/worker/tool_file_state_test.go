package worker

import (
	"context"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestRecordToolFileReadPreservesConcurrentReadsAcrossLazyStateInit(t *testing.T) {
	const readsPerBatch = 32

	workDir := t.TempDir()

	for iteration := 0; iteration < 64; iteration++ {
		ctx := &ExecutionContext{
			Context: context.Background(),
			WorkDir: workDir,
		}

		var start sync.WaitGroup
		start.Add(1)

		var wg sync.WaitGroup
		wg.Add(readsPerBatch)

		for i := 0; i < readsPerBatch; i++ {
			go func(index int) {
				defer wg.Done()
				start.Wait()
				recordToolFileRead(
					ctx,
					filepath.Join(workDir, "file-"+strconv.Itoa(index)+".txt"),
					time.Unix(int64(index+1), 0).UTC(),
					"read_file",
				)
			}(i)
		}

		start.Done()
		wg.Wait()

		state := ensureToolFileState(ctx)
		state.mu.Lock()
		recordedReads := len(state.reads)
		state.mu.Unlock()

		if recordedReads != readsPerBatch {
			t.Fatalf("expected %d recorded reads after iteration %d, got %d", readsPerBatch, iteration, recordedReads)
		}
	}
}
