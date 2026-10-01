package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

const OpenRouterTranscriptionModel = "openai/gpt-transcribe"

type TranscriptionResult struct {
	Text              string
	AudioMilliseconds int64
}

// OpenRouterTranscriptionClient keeps audio requests separate from token-based chat.
type OpenRouterTranscriptionClient struct {
	apiKey, baseURL string
	httpClient      *http.Client
}

func NewOpenRouterTranscriptionClient(key, baseURL string, client *http.Client) *OpenRouterTranscriptionClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = OpenRouterDefaultBaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &OpenRouterTranscriptionClient{apiKey: strings.TrimSpace(key), baseURL: baseURL, httpClient: client}
}

func (c *OpenRouterTranscriptionClient) Transcribe(ctx context.Context, audio []byte) (TranscriptionResult, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("model", OpenRouterTranscriptionModel); err != nil {
		return TranscriptionResult{}, err
	}
	part, err := form.CreateFormFile("file", "recording.wav")
	if err != nil {
		return TranscriptionResult{}, err
	}
	if _, err = part.Write(audio); err != nil {
		return TranscriptionResult{}, err
	}
	if err = form.Close(); err != nil {
		return TranscriptionResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", &body)
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("invalid transcription endpoint")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("transcription request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return TranscriptionResult{}, fmt.Errorf("transcription provider returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024+1))
	if err != nil || len(data) > 256*1024 {
		return TranscriptionResult{}, fmt.Errorf("invalid transcription response")
	}
	var response struct {
		Text  *string `json:"text"`
		Usage struct {
			Seconds *float64 `json:"seconds"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &response) != nil || response.Text == nil {
		return TranscriptionResult{}, fmt.Errorf("invalid transcription response")
	}
	result := TranscriptionResult{Text: strings.TrimSpace(*response.Text)}
	// OpenRouter's normalized usage has no OpenAI-specific usage.type, and
	// duration is optional. The service can measure our validated PCM input.
	if seconds := response.Usage.Seconds; seconds != nil {
		if math.IsNaN(*seconds) || math.IsInf(*seconds, 0) || *seconds <= 0 || *seconds > 3600 {
			return TranscriptionResult{}, fmt.Errorf("invalid transcription usage")
		}
		result.AudioMilliseconds = int64(math.Ceil(*seconds * 1000))
	}
	return result, nil
}
