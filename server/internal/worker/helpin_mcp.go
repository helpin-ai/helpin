package worker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/helpin-ai/helpin/server/internal/auth"
)

type HelpinMCPBridgeConfig struct {
	Command string
	BaseURL string
	Token   string
}

func BuildHelpinMCPBridgeConfig(execCtx *ExecutionContext, baseURL, tokenSecret, bridgePath string) (HelpinMCPBridgeConfig, bool) {
	if execCtx == nil || strings.TrimSpace(execCtx.RunID) == "" || strings.TrimSpace(execCtx.WorkspaceID) == "" {
		return HelpinMCPBridgeConfig{}, false
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || strings.TrimSpace(tokenSecret) == "" {
		return HelpinMCPBridgeConfig{}, false
	}
	if !strings.HasSuffix(baseURL, "/api") {
		baseURL += "/api"
	}
	token, err := auth.NewJWTManager(tokenSecret).GenerateAgentRunToolToken(execCtx.RunID, execCtx.WorkspaceID, 0)
	if err != nil {
		slog.WarnContext(execCtx.Context, "failed to generate Helpin MCP run token", "error", err, "run_id", execCtx.RunID)
		return HelpinMCPBridgeConfig{}, false
	}
	command := resolveHelpinMCPBridgeCommand(bridgePath)
	slog.InfoContext(execCtx.Context, "configured Helpin MCP bridge", "command", command, "run_id", execCtx.RunID)
	return HelpinMCPBridgeConfig{Command: command, BaseURL: baseURL, Token: token}, true
}

func resolveHelpinMCPBridgeCommand(raw string) string {
	command := strings.TrimSpace(os.ExpandEnv(raw))
	if command == "" {
		if resolved := firstExistingHelpinMCPBridgeCommand(); resolved != "" {
			return resolved
		}
		if resolved, err := exec.LookPath("helpin-mcp-bridge"); err == nil {
			return resolved
		}
		return "helpin-mcp-bridge"
	}
	if strings.ContainsAny(command, `/\`) {
		if resolved := absoluteExistingPath(command); resolved != "" {
			return resolved
		}
		if filepath.Base(command) == "helpin-mcp-bridge" {
			if resolved := firstExistingHelpinMCPBridgeCommand(); resolved != "" {
				return resolved
			}
		}
		if resolved := absolutePath(command); resolved != "" {
			return resolved
		}
		return command
	}
	if resolved, err := exec.LookPath(command); err == nil {
		return resolved
	}
	return command
}

func firstExistingHelpinMCPBridgeCommand() string {
	for _, candidate := range []string{
		"bin/helpin-mcp-bridge",
		"../bin/helpin-mcp-bridge",
		"../../bin/helpin-mcp-bridge",
		"/usr/local/bin/helpin-mcp-bridge",
	} {
		if resolved := absoluteExistingPath(candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

func validateHelpinMCPBridgeCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("Helpin MCP bridge command is empty")
	}
	if strings.ContainsAny(command, `/\`) {
		info, err := os.Stat(command)
		if err != nil {
			return fmt.Errorf("Helpin MCP bridge command not found: %s: %w", command, err)
		}
		if info.IsDir() {
			return fmt.Errorf("Helpin MCP bridge command is a directory: %s", command)
		}
		if info.Mode()&0o111 == 0 {
			return fmt.Errorf("Helpin MCP bridge command is not executable: %s", command)
		}
		return nil
	}
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("Helpin MCP bridge command not found on PATH: %s: %w", command, err)
	}
	return nil
}

func absoluteExistingPath(path string) string {
	path = absolutePath(path)
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

func absolutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return abs
}

type helpinMCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	mu     sync.Mutex
	nextID int64
}

type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newHelpinMCPClient(ctx context.Context, config HelpinMCPBridgeConfig) (*helpinMCPClient, error) {
	command := strings.TrimSpace(config.Command)
	if command == "" {
		return nil, fmt.Errorf("Helpin MCP bridge command is required")
	}
	cmd := exec.CommandContext(ctx, command)
	cmd.Env = append(os.Environ(),
		"HELPIN_API_BASE_URL="+strings.TrimSpace(config.BaseURL),
		"HELPIN_AGENT_RUN_TOOL_TOKEN="+strings.TrimSpace(config.Token),
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}
	client := &helpinMCPClient{
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewReader(stdout),
	}
	if _, err := client.request("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "helpin-native-sdk",
			"version": "1.0.0",
		},
	}, nil); err != nil {
		_ = client.Close()
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("initialize Helpin MCP bridge: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("initialize Helpin MCP bridge: %w", err)
	}
	return client, nil
}

func (c *helpinMCPClient) Close() error {
	if c == nil {
		return nil
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	return nil
}

func (c *helpinMCPClient) ListTools() ([]ToolDefinition, map[string]bool, error) {
	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if _, err := c.request("tools/list", nil, &result); err != nil {
		return nil, nil, err
	}
	defs := make([]ToolDefinition, 0, len(result.Tools))
	names := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		var schema interface{} = map[string]interface{}{"type": "object"}
		if len(tool.InputSchema) > 0 {
			var decoded interface{}
			if err := json.Unmarshal(tool.InputSchema, &decoded); err == nil && decoded != nil {
				schema = decoded
			}
		}
		defs = append(defs, ToolDefinition{
			Name:        name,
			Description: strings.TrimSpace(tool.Description),
			InputSchema: schema,
		})
		names[name] = true
	}
	return defs, names, nil
}

func (c *helpinMCPClient) CallTool(name string, input json.RawMessage) (string, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	if _, err := c.request("tools/call", map[string]any{
		"name":      name,
		"arguments": json.RawMessage(input),
	}, &result); err != nil {
		return "", err
	}
	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		if strings.TrimSpace(item.Text) != "" {
			parts = append(parts, item.Text)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if result.IsError {
		if text == "" {
			text = "MCP tool returned an error"
		}
		return "", fmt.Errorf("%s", text)
	}
	return text, nil
}

func (c *helpinMCPClient) request(method string, params interface{}, out interface{}) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("Helpin MCP client is not configured")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	id := atomic.AddInt64(&c.nextID, 1)
	payload, err := json.Marshal(mcpRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n%s", len(payload), payload); err != nil {
		return nil, err
	}
	resp, err := c.readResponse()
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("MCP %s failed: %s", method, resp.Error.Message)
	}
	if out != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return nil, err
		}
	}
	return resp.Result, nil
}

func (c *helpinMCPClient) readResponse() (*mcpResponse, error) {
	headers := textproto.MIMEHeader{}
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid MCP header %q", line)
		}
		headers.Add(strings.TrimSpace(key), strings.TrimSpace(value))
	}
	length, err := strconv.Atoi(headers.Get("Content-Length"))
	if err != nil || length <= 0 {
		return nil, fmt.Errorf("invalid MCP content length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(c.reader, body); err != nil {
		return nil, err
	}
	var resp mcpResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
