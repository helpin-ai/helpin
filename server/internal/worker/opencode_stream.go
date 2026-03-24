package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type openCodeStreamCollector struct {
	runID  string
	writer *openCodeArtifactWriter
	mu     sync.Mutex

	stdoutFull  strings.Builder
	stderrFull  strings.Builder
	stdoutChunk strings.Builder
	stderrChunk strings.Builder
	response    strings.Builder
	eventError  string
	tokensUsed  int
}

func newOpenCodeStreamCollector(runID string, writer *openCodeArtifactWriter) *openCodeStreamCollector {
	return &openCodeStreamCollector{
		runID:  runID,
		writer: writer,
	}
}

func (c *openCodeStreamCollector) AppendLine(ctx context.Context, stream, line string) {
	logLine := strings.TrimSpace(line)
	if logLine != "" {
		slog.DebugContext(ctx, "opencode stream line",
			"run_id", shortRunID(c.runID),
			"stream", stream,
			"line", truncate(logLine, 1000),
		)
	}

	c.mu.Lock()
	full, chunk := c.buffersFor(stream)
	full.WriteString(line)
	full.WriteByte('\n')
	chunk.WriteString(line)
	chunk.WriteByte('\n')
	shouldFlush := chunk.Len() >= openCodeChunkFlushBytes
	c.mu.Unlock()

	if shouldFlush {
		c.FlushStream(ctx, stream, true)
	}
}

func (c *openCodeStreamCollector) AppendResponseText(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.response.Len() > 0 {
		c.response.WriteString("\n")
	}
	c.response.WriteString(text)
}

func (c *openCodeStreamCollector) AddTokens(tokens int) {
	if tokens <= 0 {
		return
	}
	c.mu.Lock()
	c.tokensUsed += tokens
	c.mu.Unlock()
}

func (c *openCodeStreamCollector) SetEventError(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	c.mu.Lock()
	if c.eventError == "" {
		c.eventError = message
	}
	c.mu.Unlock()
}

func (c *openCodeStreamCollector) FlushLoop(ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(openCodeChunkFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.FlushPending(ctx, true)
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *openCodeStreamCollector) FlushPending(ctx context.Context, notify bool) {
	c.FlushStream(ctx, "stdout", notify)
	c.FlushStream(ctx, "stderr", notify)
}

func (c *openCodeStreamCollector) FlushStream(ctx context.Context, stream string, notify bool) {
	c.mu.Lock()
	_, chunk := c.buffersFor(stream)
	content := chunk.String()
	chunk.Reset()
	c.mu.Unlock()

	if content == "" {
		return
	}
	c.writer.Save(ctx, "opencode_"+stream+"_chunk", "text", content, notify)
}

func (c *openCodeStreamCollector) Outputs() (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stdoutFull.String(), c.stderrFull.String()
}

func (c *openCodeStreamCollector) ResponseText() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.response.String()
}

func (c *openCodeStreamCollector) EventError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.eventError
}

func (c *openCodeStreamCollector) TokensUsed() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokensUsed
}

func (c *openCodeStreamCollector) buffersFor(stream string) (*strings.Builder, *strings.Builder) {
	if stream == "stderr" {
		return &c.stderrFull, &c.stderrChunk
	}
	return &c.stdoutFull, &c.stdoutChunk
}

func consumeOpenCodeTextStream(reader io.Reader, stream string, ctx context.Context, collector *openCodeStreamCollector, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), openCodeScannerBufferSize)
	for scanner.Scan() {
		collector.AppendLine(ctx, stream, stripANSI(scanner.Text()))
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		errCh <- fmt.Errorf("read %s: %w", stream, err)
		return
	}
	errCh <- nil
}

func consumeOpenCodeJSONStream(reader io.Reader, ctx context.Context, collector *openCodeStreamCollector, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), openCodeScannerBufferSize)
	for scanner.Scan() {
		line := strings.TrimSpace(stripANSI(scanner.Text()))
		if line == "" {
			continue
		}

		displayLine, responseText, tokensUsed, eventErr, err := parseOpenCodeJSONEvent(line)
		if err != nil {
			slog.DebugContext(ctx, "treating non-json opencode stdout line as plain text",
				"error", err,
				"run_id", shortRunID(collector.runID),
			)
			collector.AppendLine(ctx, "stdout", line)
			continue
		}
		if strings.TrimSpace(displayLine) != "" {
			collector.AppendLine(ctx, "stdout", displayLine)
		}
		if responseText != "" {
			collector.AppendResponseText(responseText)
		}
		if tokensUsed > 0 {
			collector.AddTokens(tokensUsed)
		}
		if eventErr != "" {
			collector.SetEventError(eventErr)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		errCh <- fmt.Errorf("read stdout: %w", err)
		return
	}
	errCh <- nil
}

func parseOpenCodeJSONEvent(line string) (displayLine, responseText string, tokensUsed int, eventErr string, err error) {
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", "", 0, "", fmt.Errorf("unmarshal event line %q: %w", truncate(line, 200), err)
	}

	eventType := lookupString(event, "type")
	part, _ := event["part"].(map[string]any)
	switch eventType {
	case "text":
		responseText = strings.TrimSpace(lookupString(part, "text"))
		return responseText, responseText, 0, "", nil
	case "tool_use":
		return renderOpenCodeToolUse(part), "", 0, "", nil
	case "step_start":
		return renderOpenCodeStepStart(part), "", 0, "", nil
	case "step_finish":
		tokensUsed = openCodeTokensFromPart(part)
		return renderOpenCodeStepFinish(part, tokensUsed), "", tokensUsed, "", nil
	case "error":
		eventErr = firstNonEmptyText(
			lookupString(event, "error"),
			lookupString(event, "message"),
			lookupString(part, "error"),
			lookupString(part, "message"),
			renderOpenCodeRawPart(part),
		)
		return strings.TrimSpace(firstNonEmptyText("Error: "+strings.TrimSpace(eventErr), eventErr)), "", 0, strings.TrimSpace(eventErr), nil
	default:
		return renderOpenCodeGenericEvent(eventType, part), "", 0, "", nil
	}
}
