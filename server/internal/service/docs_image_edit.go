package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const falNanoBanana2EditEndpoint = "https://fal.run/fal-ai/nano-banana-2/edit"

// DocsImageEditService is the reusable server-side fal.ai image editing tool.
type DocsImageEditService struct {
	falAPIKey   string
	attachments *PMAttachmentService
	client      *http.Client
}

// NewDocsImageEditService creates an image editing tool backed by fal.ai.
func NewDocsImageEditService(falAPIKey string, attachments *PMAttachmentService) *DocsImageEditService {
	return &DocsImageEditService{falAPIKey: strings.TrimSpace(falAPIKey), attachments: attachments, client: &http.Client{Timeout: 90 * time.Second}}
}

type falImageEditResult struct {
	Images []struct {
		URL         string `json:"url"`
		ContentType string `json:"content_type"`
		FileName    string `json:"file_name"`
	} `json:"images"`
}

// Edit applies a prompt to a docs image and stores the output as a new attachment.
func (s *DocsImageEditService) Edit(ctx context.Context, workspaceID, userID, documentID string, req model.EditDocsImageRequest) (*model.EditDocsImageResponse, error) {
	if s == nil || s.falAPIKey == "" {
		return nil, fmt.Errorf("AI image editing is not configured")
	}
	if s.attachments == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("describe the changes you want to make")
	}
	sourceURL, err := s.attachments.SourceURL(ctx, req.SourceAttachmentID, workspaceID, documentID)
	if err != nil {
		return nil, err
	}
	imageURLs := []string{sourceURL}
	if annotation := strings.TrimSpace(req.AnnotationDataURL); annotation != "" {
		if !strings.HasPrefix(annotation, "data:image/png;base64,") || len(annotation) > 8<<20 {
			return nil, fmt.Errorf("annotation must be a PNG image smaller than 8MB")
		}
		imageURLs = append(imageURLs, annotation)
		prompt += " The second reference image is an annotated copy of the first. Apply the requested change only at the red-marked location and preserve all unmarked content."
	}
	payload, err := json.Marshal(map[string]any{"prompt": prompt, "image_urls": imageURLs, "num_images": 1, "aspect_ratio": "auto", "resolution": "1K", "output_format": "png"})
	if err != nil {
		return nil, fmt.Errorf("encode image edit request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, falNanoBanana2EditEndpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("create image edit request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Key "+s.falAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request image edit: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("image provider could not complete this edit")
	}
	var result falImageEditResult
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode image edit response: %w", err)
	}
	if len(result.Images) == 0 || strings.TrimSpace(result.Images[0].URL) == "" {
		return nil, fmt.Errorf("image provider returned no image")
	}
	image := result.Images[0]
	download, err := http.NewRequestWithContext(ctx, http.MethodGet, image.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create edited-image download: %w", err)
	}
	asset, err := s.client.Do(download)
	if err != nil {
		return nil, fmt.Errorf("download edited image: %w", err)
	}
	defer asset.Body.Close()
	if asset.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download edited image: provider returned %d", asset.StatusCode)
	}
	contentType := image.ContentType
	if contentType == "" {
		contentType = asset.Header.Get("Content-Type")
	}
	if !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("image provider returned an unsupported file")
	}
	filename := filepath.Base(image.FileName)
	if filename == "." || filename == "" {
		filename = "ai-edit-" + uuid.NewString() + ".png"
	}
	created, err := s.attachments.CreateImported(ctx, model.CreateAttachmentRequest{EntityType: "editor_upload", EntityID: documentID, FileName: filename, FileSize: asset.ContentLength, ContentType: contentType}, workspaceID, userID, asset.Body)
	if err != nil {
		return nil, fmt.Errorf("store edited image: %w", err)
	}
	return &model.EditDocsImageResponse{AttachmentID: created.Attachment.ID}, nil
}
