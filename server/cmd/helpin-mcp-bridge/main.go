package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type gatewayToolList struct {
	Tools []gatewayTool `json:"tools"`
}

type gatewayTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type gatewayCallRequest struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
}

type frameFormat string

const (
	frameJSONLine      frameFormat = "json_line"
	frameContentLength frameFormat = "content_length"
)

func main() {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("HELPIN_API_BASE_URL")), "/")
	token := strings.TrimSpace(os.Getenv("HELPIN_AGENT_RUN_TOOL_TOKEN"))
	if baseURL == "" || token == "" {
		fmt.Fprintln(os.Stderr, "HELPIN_API_BASE_URL and HELPIN_AGENT_RUN_TOOL_TOKEN are required")
		os.Exit(1)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	reader := bufio.NewReader(os.Stdin)
	for {
		msg, format, err := readMCPMessage(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			fmt.Fprintln(os.Stderr, err)
			return
		}
		var req rpcRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			writeMCPResponse(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}}, format)
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		resp := handleRequest(client, baseURL, token, req)
		writeMCPResponse(resp, format)
	}
}

func handleRequest(client *http.Client, baseURL, token string, req rpcRequest) rpcResponse {
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{"name": "helpin", "version": "0.1.0"},
		}
	case "tools/list":
		tools, err := listTools(client, baseURL, token)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
			return resp
		}
		mcpTools := make([]map[string]any, 0, len(tools.Tools))
		for _, tool := range tools.Tools {
			var schema any = map[string]any{"type": "object", "properties": map[string]any{}}
			if len(tool.InputSchema) > 0 {
				_ = json.Unmarshal(tool.InputSchema, &schema)
			}
			mcpTools = append(mcpTools, map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": schema,
			})
		}
		resp.Result = map[string]any{"tools": mcpTools}
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &rpcError{Code: -32602, Message: "invalid tools/call params"}
			return resp
		}
		result, err := callTool(client, baseURL, token, params)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
			return resp
		}
		resp.Result = result
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	return resp
}

func listTools(client *http.Client, baseURL, token string) (*gatewayToolList, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/agent-run-tools/tools", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list tools failed: %s", strings.TrimSpace(string(body)))
	}
	var out gatewayToolList
	if err := decodeGatewayJSON(resp, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func callTool(client *http.Client, baseURL, token string, params toolCallParams) (map[string]any, error) {
	args := params.Arguments
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage(`{}`)
	}
	payload, _ := json.Marshal(gatewayCallRequest{ToolName: params.Name, Input: args})
	req, err := http.NewRequest(http.MethodPost, baseURL+"/agent-run-tools/call", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("call tool failed: %s", strings.TrimSpace(string(body)))
	}
	var out map[string]any
	if err := decodeGatewayJSON(resp, body, &out); err != nil {
		return nil, err
	}
	result := map[string]any{
		"content": out["content"],
	}
	if isError, _ := out["is_error"].(bool); isError {
		result["isError"] = true
	}
	return result, nil
}

func decodeGatewayJSON(resp *http.Response, body []byte, out any) error {
	if err := json.Unmarshal(body, out); err != nil {
		preview := strings.TrimSpace(string(body))
		if len(preview) > 240 {
			preview = preview[:240] + "..."
		}
		contentType := ""
		url := ""
		if resp != nil {
			contentType = resp.Header.Get("Content-Type")
			if resp.Request != nil && resp.Request.URL != nil {
				url = resp.Request.URL.String()
			}
		}
		return fmt.Errorf("Helpin API returned non-JSON response from %s (content-type=%q): %s", url, contentType, preview)
	}
	return nil
}

func readMCPMessage(r *bufio.Reader) ([]byte, frameFormat, error) {
	contentLength := -1
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, frameJSONLine, err
	}
	line = strings.TrimRight(line, "\r\n")
	if strings.HasPrefix(strings.TrimSpace(line), "{") {
		return []byte(strings.TrimSpace(line)), frameJSONLine, nil
	}
	for {
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, frameContentLength, err
			}
			contentLength = n
		}
		line, err = r.ReadString('\n')
		if err != nil {
			return nil, frameContentLength, err
		}
	}
	if contentLength < 0 {
		return nil, frameContentLength, fmt.Errorf("missing Content-Length")
	}
	buf := make([]byte, contentLength)
	_, err = io.ReadFull(r, buf)
	return buf, frameContentLength, err
}

func writeMCPResponse(resp rpcResponse, format frameFormat) {
	payload, _ := json.Marshal(resp)
	if format == frameContentLength {
		fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
		return
	}
	fmt.Fprintf(os.Stdout, "%s\n", payload)
}
