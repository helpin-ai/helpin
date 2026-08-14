package meetingcapture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxProviderResponseBytes = 20 << 20

type httpClient struct {
	baseURL string
	client  *http.Client
	applyAuth func(*http.Request)
}

func newHTTPClient(baseURL string, client *http.Client, applyAuth func(*http.Request)) *httpClient {
	if client == nil {
		client = &http.Client{}
	}
	return &httpClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client: client,
		applyAuth: applyAuth,
	}
}

func (c *httpClient) doJSON(ctx context.Context, method, path string, body, target interface{}) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal provider request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("create provider request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.applyAuth != nil {
		c.applyAuth(req)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("call provider: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read provider response: %w", err)
	}
	if len(payload) > maxProviderResponseBytes {
		return fmt.Errorf("provider response exceeds size limit")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &HTTPError{StatusCode: resp.StatusCode, Body: sanitizeProviderError(payload)}
	}
	if target == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode provider response: %w", err)
	}
	return nil
}

func (c *httpClient) downloadJSON(ctx context.Context, downloadURL string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create artifact request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("download provider artifact: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("download provider artifact: status %d", resp.StatusCode)
		}
		return &HTTPError{StatusCode: resp.StatusCode, Body: sanitizeProviderError(payload)}
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxProviderResponseBytes))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode provider artifact: %w", err)
	}
	return nil
}

// HTTPError is a sanitized non-2xx provider response.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("meeting provider returned status %d: %s", e.StatusCode, e.Body)
}

func sanitizeProviderError(payload []byte) string {
	text := strings.TrimSpace(string(payload))
	if len(text) > 500 {
		text = text[:500]
	}
	if text == "" {
		return "empty response"
	}
	return text
}
