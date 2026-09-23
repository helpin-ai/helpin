package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

// OpenAI Images API models used by agent image tools.
const (
	OpenAIImageModelFast    = "gpt-image-2.5-flare"
	OpenAIImageModelQuality = "gpt-image-2.5-sunburst"
)

// OpenAIImagesClient calls the OpenAI Images API (generations and edits).
// It is deliberately separate from chat providers: images are billed and
// governed as their own AI action modality.
type OpenAIImagesClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewOpenAIImagesClient builds a client. An empty baseURL uses the public API.
func NewOpenAIImagesClient(apiKey, baseURL string, httpClient *http.Client) *OpenAIImagesClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if httpClient == nil {
		// Large, high-quality images can take several minutes.
		httpClient = &http.Client{Timeout: 4 * time.Minute}
	}
	return &OpenAIImagesClient{apiKey: strings.TrimSpace(apiKey), baseURL: baseURL, httpClient: httpClient}
}

// ImageInput is one source image for an edit.
type ImageInput struct {
	FileName    string
	ContentType string
	Data        []byte
}

// ImageGenerateRequest describes a text-to-image request.
type ImageGenerateRequest struct {
	Model        string
	Prompt       string
	Size         string // "auto" or WIDTHxHEIGHT
	Quality      string // low, medium, high, xhigh, max, auto
	Background   string // transparent, opaque, auto
	OutputFormat string // png, jpeg, webp
}

// ImageEditRequest describes an edit of one or more source images.
type ImageEditRequest struct {
	ImageGenerateRequest
	Images []ImageInput
	// Mask is an optional PNG with an alpha channel, the same size as the first
	// image; transparent pixels mark the region to change.
	Mask *ImageInput
}

// ImageUsage is the token usage the Images API reports.
type ImageUsage struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	TotalTokens       int `json:"total_tokens"`
	InputTokenDetails struct {
		TextTokens  int `json:"text_tokens"`
		ImageTokens int `json:"image_tokens"`
	} `json:"input_tokens_details"`
}

// ImageResult is the first image returned by the API.
type ImageResult struct {
	Data          []byte
	OutputFormat  string
	Size          string
	Quality       string
	Background    string
	RevisedPrompt string
	Usage         ImageUsage
}

type imagesAPIResponse struct {
	Data []struct {
		B64JSON       string `json:"b64_json"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"data"`
	OutputFormat string     `json:"output_format"`
	Size         string     `json:"size"`
	Quality      string     `json:"quality"`
	Background   string     `json:"background"`
	Usage        ImageUsage `json:"usage"`
	Error        *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ImageAPIError is a non-2xx response from the Images API.
type ImageAPIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ImageAPIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("openai images: %s (%d %s)", e.Message, e.StatusCode, e.Code)
	}
	return fmt.Sprintf("openai images: %s (%d)", e.Message, e.StatusCode)
}

// Generate creates an image from a text prompt.
func (c *OpenAIImagesClient) Generate(ctx context.Context, req ImageGenerateRequest) (*ImageResult, error) {
	body := map[string]any{"model": req.Model, "prompt": req.Prompt, "n": 1}
	for key, value := range map[string]string{"size": req.Size, "quality": req.Quality, "background": req.Background, "output_format": req.OutputFormat} {
		if strings.TrimSpace(value) != "" {
			body[key] = value
		}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/images/generations", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return c.do(httpReq)
}

// Edit changes one or more source images according to the prompt.
func (c *OpenAIImagesClient) Edit(ctx context.Context, req ImageEditRequest) (*ImageResult, error) {
	if len(req.Images) == 0 {
		return nil, fmt.Errorf("openai images: at least one source image is required")
	}
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	fields := map[string]string{"model": req.Model, "prompt": req.Prompt, "n": "1", "size": req.Size,
		"quality": req.Quality, "background": req.Background, "output_format": req.OutputFormat}
	for key, value := range fields {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if err := form.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	for _, image := range req.Images {
		if err := writeImagePart(form, "image[]", image); err != nil {
			return nil, err
		}
	}
	if req.Mask != nil {
		if err := writeImagePart(form, "mask", *req.Mask); err != nil {
			return nil, err
		}
	}
	if err := form.Close(); err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/images/edits", &buf)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", form.FormDataContentType())
	return c.do(httpReq)
}

func writeImagePart(form *multipart.Writer, field string, image ImageInput) error {
	header := make(textproto.MIMEHeader)
	name := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(image.FileName)
	if name == "" {
		name = "image.png"
	}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, name))
	header.Set("Content-Type", image.ContentType)
	part, err := form.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = part.Write(image.Data)
	return err
}

func (c *OpenAIImagesClient) do(req *http.Request) (*ImageResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("openai images: API key is required")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai images: %w", err)
	}
	defer resp.Body.Close()
	// Base64 images are large; cap reads well above the largest output.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
	if err != nil {
		return nil, fmt.Errorf("openai images: read response: %w", err)
	}
	var decoded imagesAPIResponse
	decodeErr := json.Unmarshal(raw, &decoded)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		apiErr := &ImageAPIError{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
		if decodeErr == nil && decoded.Error != nil {
			apiErr.Message, apiErr.Code = decoded.Error.Message, decoded.Error.Code
		}
		return nil, apiErr
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("openai images: decode response: %w", decodeErr)
	}
	if len(decoded.Data) == 0 || decoded.Data[0].B64JSON == "" {
		return nil, fmt.Errorf("openai images: response contained no image")
	}
	data, err := base64.StdEncoding.DecodeString(decoded.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("openai images: decode image: %w", err)
	}
	return &ImageResult{
		Data: data, OutputFormat: decoded.OutputFormat, Size: decoded.Size, Quality: decoded.Quality,
		Background: decoded.Background, RevisedPrompt: decoded.Data[0].RevisedPrompt, Usage: decoded.Usage,
	}, nil
}
