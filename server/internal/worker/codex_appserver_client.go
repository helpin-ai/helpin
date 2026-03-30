package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const codexRPCScannerBufferSize = 2 * 1024 * 1024

type codexRPCMessage struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *codexRPCError  `json:"error,omitempty"`
}

type codexRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type codexAppServerClient struct {
	commandPath string
	workDir     string
	env         []string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan codexRPCMessage
	done   chan error
	buffer []codexRPCMessage

	nextRequestID atomic.Int64

	stderrMu sync.Mutex
	stderr   strings.Builder
}

func newCodexAppServerClient(commandPath, workDir string, env []string) *codexAppServerClient {
	client := &codexAppServerClient{
		commandPath: strings.TrimSpace(commandPath),
		workDir:     workDir,
		env:         append([]string(nil), env...),
		lines:       make(chan codexRPCMessage, 128),
		done:        make(chan error, 2),
	}
	client.nextRequestID.Store(1)
	return client
}

func (c *codexAppServerClient) Start(ctx context.Context) error {
	if c.cmd != nil {
		return nil
	}

	bin, prefixArgs, launchDir, err := resolveCodexLaunch(c.commandPath, c.workDir)
	if err != nil {
		return err
	}
	args := append(prefixArgs, "app-server", "--listen", "stdio://")

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = launchDir
	cmd.Env = c.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 10 * time.Second

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create codex app-server stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create codex app-server stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create codex app-server stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start codex app-server: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin

	go c.readStdout(stdout)
	go c.readStderr(stderr)
	go func() {
		c.done <- cmd.Wait()
	}()

	return nil
}

func resolveCodexLaunch(commandPath, fallbackWorkDir string) (bin string, prefixArgs []string, workDir string, err error) {
	path := strings.TrimSpace(commandPath)
	if path == "" {
		return "codex", nil, fallbackWorkDir, nil
	}
	if strings.HasSuffix(path, ".js") {
		return "node", []string{path}, fallbackWorkDir, nil
	}
	info, statErr := os.Stat(path)
	if statErr == nil && info.IsDir() {
		manifest := filepath.Join(path, "codex-rs", "Cargo.toml")
		if _, manifestErr := os.Stat(manifest); manifestErr == nil {
			return "cargo", []string{"run", "--quiet", "--manifest-path", manifest, "-p", "codex-cli", "--"}, path, nil
		}
	}
	return path, nil, fallbackWorkDir, nil
}

func (c *codexAppServerClient) Close() error {
	if c == nil || c.cmd == nil {
		return nil
	}
	cmd := c.cmd
	c.cmd = nil

	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}

	select {
	case err := <-c.done:
		if err != nil && !errors.Is(err, exec.ErrWaitDelay) && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
	case <-time.After(2 * time.Second):
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		select {
		case err := <-c.done:
			if err != nil && !errors.Is(err, os.ErrProcessDone) {
				return err
			}
		default:
		}
	}

	return nil
}

func (c *codexAppServerClient) Initialize(ctx context.Context) error {
	_, err := c.Request(ctx, "initialize", map[string]any{
		"clientInfo": map[string]any{
			"name":    "helpin_coding_runtime",
			"title":   "Helpin Coding Runtime",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{
			"experimentalApi": true,
		},
	})
	if err != nil {
		return err
	}
	return c.Notify(ctx, "initialized", map[string]any{})
}

func (c *codexAppServerClient) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	requestID := c.nextID()
	if err := c.write(ctx, map[string]any{
		"id":     requestID,
		"method": method,
		"params": jsonObjectOrEmpty(params),
	}); err != nil {
		return nil, err
	}

	for {
		msg, err := c.Next(ctx)
		if err != nil {
			return nil, err
		}
		if msg.Method != "" {
			c.buffer = append(c.buffer, msg)
			continue
		}
		if !jsonRawEqual(msg.ID, requestID) {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("codex app-server %s failed (%d): %s", method, msg.Error.Code, strings.TrimSpace(msg.Error.Message))
		}
		return msg.Result, nil
	}
}

func (c *codexAppServerClient) Notify(ctx context.Context, method string, params any) error {
	return c.write(ctx, map[string]any{
		"method": method,
		"params": jsonObjectOrEmpty(params),
	})
}

func (c *codexAppServerClient) Respond(ctx context.Context, id json.RawMessage, result any) error {
	return c.write(ctx, map[string]any{
		"id":     jsonRawToValue(id),
		"result": result,
	})
}

func (c *codexAppServerClient) Next(ctx context.Context) (codexRPCMessage, error) {
	if len(c.buffer) > 0 {
		msg := c.buffer[0]
		c.buffer = c.buffer[1:]
		return msg, nil
	}

	select {
	case msg, ok := <-c.lines:
		if !ok {
			select {
			case err := <-c.done:
				if err == nil {
					return codexRPCMessage{}, io.EOF
				}
				return codexRPCMessage{}, err
			default:
				return codexRPCMessage{}, io.EOF
			}
		}
		return msg, nil
	case err := <-c.done:
		if err == nil {
			return codexRPCMessage{}, io.EOF
		}
		return codexRPCMessage{}, err
	case <-ctx.Done():
		return codexRPCMessage{}, ctx.Err()
	}
}

func (c *codexAppServerClient) Stderr() string {
	c.stderrMu.Lock()
	defer c.stderrMu.Unlock()
	return c.stderr.String()
}

func (c *codexAppServerClient) write(ctx context.Context, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal codex app-server payload: %w", err)
	}
	data = append(data, '\n')

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("write codex app-server payload: %w", err)
	}
	return nil
}

func (c *codexAppServerClient) readStdout(stdout io.Reader) {
	defer close(c.lines)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), codexRPCScannerBufferSize)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg codexRPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		c.lines <- msg
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		c.done <- err
	}
}

func (c *codexAppServerClient) readStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), codexRPCScannerBufferSize)
	for scanner.Scan() {
		line := stripANSI(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		c.stderrMu.Lock()
		c.stderr.WriteString(line)
		c.stderr.WriteByte('\n')
		c.stderrMu.Unlock()
	}
}

func (c *codexAppServerClient) nextID() json.RawMessage {
	value := c.nextRequestID.Add(1) - 1
	return json.RawMessage(fmt.Sprintf("%d", value))
}

func jsonObjectOrEmpty(value any) any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func jsonRawEqual(left, right json.RawMessage) bool {
	return strings.TrimSpace(string(left)) == strings.TrimSpace(string(right))
}

func jsonRawToValue(raw json.RawMessage) any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return trimmed
	}
	return value
}
