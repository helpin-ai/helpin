package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Typed public MCP upload error codes.
const (
	MCPErrorCodeUploadNotFound         = "UPLOAD_NOT_FOUND"
	MCPErrorCodeUploadSizeMismatch     = "UPLOAD_SIZE_MISMATCH"
	MCPErrorCodeUnsupportedContentType = "UNSUPPORTED_CONTENT_TYPE"
	MCPErrorCodeURLNotPublic           = "URL_NOT_PUBLIC"
	MCPErrorCodeURLFetchFailed         = "URL_FETCH_FAILED"
)

const (
	_mcpDocumentImageMaxBytes = 20 << 20
	_mcpImageFromURLMaxBytes  = dockChatExternalImageMaxBytes
)

var _mcpDocumentImageTypes = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}

type mcpAttachmentService interface {
	Create(context.Context, model.CreateAttachmentRequest, string, string) (*model.AttachmentResponse, error)
	CreateImported(context.Context, model.CreateAttachmentRequest, string, string, io.Reader) (*model.AttachmentResponse, error)
	Get(context.Context, string) (*model.PMAttachment, error)
	VerifyAndConfirmUpload(context.Context, string) error
}

// mcpImageFetcher downloads a public image and returns its content type and bytes.
type mcpImageFetcher func(ctx context.Context, rawURL string) (string, []byte, error)

// SetAttachments wires attachment uploads used by the document image tools.
func (s *MCPService) SetAttachments(attachments *PMAttachmentService) {
	if attachments != nil {
		s.attachments = attachments
	}
	if s.fetchImage == nil {
		client := newDockChatExternalMediaClient()
		s.fetchImage = func(ctx context.Context, rawURL string) (string, []byte, error) {
			return fetchMCPPublicImage(ctx, client, rawURL)
		}
	}
}

func uploadMCPToolDefinitions() []MCPToolDefinition {
	object := func(properties map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	documentID := map[string]any{"type": "string", "minLength": 1}
	fileName := map[string]any{"type": "string", "minLength": 1, "maxLength": 200}
	docsWrite := func(definition MCPToolDefinition) MCPToolDefinition {
		definition.Toolset, definition.Scope = MCPToolsetDocs, MCPScopeDocsWrite
		definition.Permission, definition.Module = authorization.PermDocsEdit, model.ModuleDocs
		definition.Mutating, definition.IdempotentHint = true, true
		return definition
	}
	return []MCPToolDefinition{
		docsWrite(MCPToolDefinition{
			Name:  "prepare_document_image_upload",
			Title: "Prepare document image upload",
			Description: "Start a direct image upload for a document. PUT the file bytes to upload_url with the " +
				"returned headers, call complete_document_image_upload, then insert the returned markdown " +
				"with insert_document_block or edit_document. PNG, JPEG, WebP, or GIF up to 20 MB.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"document_id":  documentID,
				"file_name":    fileName,
				"content_type": map[string]any{"type": "string", "enum": _mcpDocumentImageTypes},
				"size_bytes":   map[string]any{"type": "integer", "minimum": 1, "maximum": _mcpDocumentImageMaxBytes},
			}, "document_id", "file_name", "content_type", "size_bytes")),
		}),
		docsWrite(MCPToolDefinition{
			Name:  "complete_document_image_upload",
			Title: "Complete document image upload",
			Description: "Confirm a direct image upload after the PUT succeeds. Helpin checks that the stored " +
				"file exists and matches the declared size before the image can be used.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"attachment_id": map[string]any{"type": "string", "minLength": 1},
				"alt_text":      map[string]any{"type": "string", "maxLength": 300, "description": "Alt text used in the returned markdown."},
			}, "attachment_id")),
		}),
		docsWrite(MCPToolDefinition{
			Name:  "upload_document_image_from_url",
			Title: "Upload document image from URL",
			Description: "Copy a public HTTPS image into Helpin for a document and return markdown to insert. " +
				"Private, local, and link-local addresses are refused. PNG, JPEG, WebP, or GIF up to 10 MB.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"document_id": documentID,
				"url":         map[string]any{"type": "string", "minLength": 1, "maxLength": 2048},
				"file_name":   fileName,
				"alt_text":    map[string]any{"type": "string", "maxLength": 300},
			}, "document_id", "url")),
		}),
	}
}

// executeUploadMCPTool runs a document image upload tool. handled is false
// when name is not an upload tool.
func (s *MCPService) executeUploadMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	name string,
	arguments json.RawMessage,
) (result *MCPToolResult, handled bool, err error) {
	switch name {
	case "prepare_document_image_upload", "complete_document_image_upload", "upload_document_image_from_url":
	default:
		return nil, false, nil
	}
	if s.attachments == nil {
		return nil, true, ErrMCPForbidden
	}
	var input struct {
		DocumentID   string `json:"document_id"`
		AttachmentID string `json:"attachment_id"`
		FileName     string `json:"file_name"`
		ContentType  string `json:"content_type"`
		SizeBytes    int64  `json:"size_bytes"`
		URL          string `json:"url"`
		AltText      string `json:"alt_text"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, true, err
	}
	switch name {
	case "prepare_document_image_upload":
		result, err = s.prepareMCPDocumentImageUpload(ctx, principal, actor, input.DocumentID, input.FileName, input.ContentType, input.SizeBytes)
	case "complete_document_image_upload":
		result, err = s.completeMCPDocumentImageUpload(ctx, principal, actor, input.AttachmentID, input.AltText)
	case "upload_document_image_from_url":
		result, err = s.uploadMCPDocumentImageFromURL(ctx, principal, actor, input.DocumentID, input.URL, input.FileName, input.AltText)
	}
	return result, true, err
}

func (s *MCPService) prepareMCPDocumentImageUpload(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	documentID, fileName, contentType string,
	size int64,
) (*MCPToolResult, error) {
	document, err := s.requireMCPEditableDocument(ctx, principal, actor, documentID)
	if err != nil {
		return nil, err
	}
	if !isMCPDocumentImageType(contentType) {
		return nil, newMCPToolError(MCPErrorCodeUnsupportedContentType, "Use PNG, JPEG, WebP, or GIF.")
	}
	response, err := s.attachments.Create(ctx, model.CreateAttachmentRequest{
		EntityType: entityTypeEditorUpload, EntityID: document.ID,
		FileName: sanitizeMCPFileName(fileName, contentType), FileSize: size, ContentType: contentType, Private: true,
	}, principal.WorkspaceID, principal.UserID)
	if err != nil {
		return nil, err
	}
	return &MCPToolResult{
		Summary: "Upload prepared. PUT the file to upload_url, then call complete_document_image_upload.",
		Data: map[string]any{
			"attachment_id": response.Attachment.ID,
			"upload_url":    response.URL,
			"method":        http.MethodPut,
			"headers":       map[string]string{"Content-Type": contentType},
			"image_src":     mcpAttachmentImageSrc(response.Attachment.ID),
		},
	}, nil
}

func (s *MCPService) completeMCPDocumentImageUpload(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	attachmentID, altText string,
) (*MCPToolResult, error) {
	attachment, err := s.attachments.Get(ctx, strings.TrimSpace(attachmentID))
	if err != nil {
		return nil, err
	}
	if attachment == nil || attachment.WorkspaceID != principal.WorkspaceID ||
		attachment.EntityType != entityTypeEditorUpload || attachment.UploadedByID != principal.UserID {
		return nil, ErrMCPNotFound
	}
	if _, err := s.requireMCPEditableDocument(ctx, principal, actor, attachment.EntityID); err != nil {
		return nil, err
	}
	if err := s.attachments.VerifyAndConfirmUpload(ctx, attachment.ID); err != nil {
		switch {
		case errors.Is(err, ErrAttachmentUploadMissing):
			return nil, newMCPToolError(MCPErrorCodeUploadNotFound, "No uploaded file was found. PUT the file to upload_url first.")
		case errors.Is(err, ErrAttachmentUploadSizeMismatch):
			return nil, newMCPToolError(MCPErrorCodeUploadSizeMismatch, "The uploaded file size does not match size_bytes. Prepare a new upload.")
		default:
			return nil, err
		}
	}
	return mcpDocumentImageResult(attachment.ID, attachment.EntityID, altText), nil
}

func (s *MCPService) uploadMCPDocumentImageFromURL(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	documentID, rawURL, fileName, altText string,
) (*MCPToolResult, error) {
	document, err := s.requireMCPEditableDocument(ctx, principal, actor, documentID)
	if err != nil {
		return nil, err
	}
	if !isDockChatExternalMediaURL(rawURL) {
		return nil, newMCPToolError(MCPErrorCodeURLNotPublic, "Use a public HTTPS image URL.")
	}
	if s.fetchImage == nil {
		return nil, ErrMCPForbidden
	}
	contentType, body, err := s.fetchImage(ctx, strings.TrimSpace(rawURL))
	if err != nil {
		var toolErr *MCPToolError
		if errors.As(err, &toolErr) {
			return nil, err
		}
		return nil, newMCPToolError(MCPErrorCodeURLFetchFailed, "The image could not be downloaded from a public address.")
	}
	if fileName == "" {
		fileName = path.Base(strings.Split(strings.TrimSpace(rawURL), "?")[0])
	}
	response, err := s.attachments.CreateImported(ctx, model.CreateAttachmentRequest{
		EntityType: entityTypeEditorUpload, EntityID: document.ID,
		FileName: sanitizeMCPFileName(fileName, contentType), FileSize: int64(len(body)), ContentType: contentType, Private: true,
	}, principal.WorkspaceID, principal.UserID, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return mcpDocumentImageResult(response.Attachment.ID, document.ID, altText), nil
}

func (s *MCPService) requireMCPEditableDocument(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	documentID string,
) (*model.DocsDocument, error) {
	document, err := s.accessibleMCPDocument(ctx, principal, actor, strings.TrimSpace(documentID))
	if err != nil {
		return nil, err
	}
	if document == nil {
		return nil, ErrMCPNotFound
	}
	if document.IsLocked {
		return nil, newMCPToolError(MCPErrorCodeDocumentLocked, "The document is locked. Unlock it in Helpin first.")
	}
	return document, nil
}

// fetchMCPPublicImage downloads an image through the SSRF-safe media client:
// HTTPS only, every resolved address checked at dial time, and bounded redirects.
func fetchMCPPublicImage(ctx context.Context, client *http.Client, rawURL string) (string, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, err
	}
	request.Header.Set("Accept", strings.Join(_mcpDocumentImageTypes, ","))
	request.Header.Set("User-Agent", "Helpin-MCP-Image/1.0")
	response, err := client.Do(request)
	if err != nil {
		return "", nil, fmt.Errorf("fetch image: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("fetch image: status %d", response.StatusCode)
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if !isMCPDocumentImageType(contentType) {
		return "", nil, newMCPToolError(MCPErrorCodeUnsupportedContentType, "The URL is not a PNG, JPEG, WebP, or GIF image.")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, _mcpImageFromURLMaxBytes+1))
	if err != nil {
		return "", nil, fmt.Errorf("read image: %w", err)
	}
	if len(body) > _mcpImageFromURLMaxBytes {
		return "", nil, newMCPToolError(MCPErrorCodeUnsupportedContentType, "The image is larger than 10 MB.")
	}
	if err := validateDockChatMediaSignature(contentType, body); err != nil {
		return "", nil, newMCPToolError(MCPErrorCodeUnsupportedContentType, "The file content does not match its image type.")
	}
	return contentType, body, nil
}

func isMCPDocumentImageType(contentType string) bool {
	for _, allowed := range _mcpDocumentImageTypes {
		if strings.EqualFold(strings.TrimSpace(contentType), allowed) {
			return true
		}
	}
	return false
}

func sanitizeMCPFileName(name, contentType string) string {
	name = strings.TrimSpace(path.Base(strings.ReplaceAll(name, "\\", "/")))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`/:*?"<>|`, r) {
			return '-'
		}
		return r
	}, name)
	if name == "" || name == "." || name == "/" {
		name = "image"
	}
	if path.Ext(name) == "" {
		name += "." + strings.TrimPrefix(contentType, "image/")
	}
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	return name
}

func mcpAttachmentImageSrc(attachmentID string) string {
	return "/api/pm/attachments/" + attachmentID + "/content"
}

func mcpDocumentImageResult(attachmentID, documentID, altText string) *MCPToolResult {
	src := mcpAttachmentImageSrc(attachmentID)
	alt := strings.NewReplacer("[", "", "]", "", "\n", " ").Replace(strings.TrimSpace(altText))
	return &MCPToolResult{
		Summary: "Image ready. Insert the markdown into the document with insert_document_block or edit_document.",
		Data: map[string]any{
			"attachment_id": attachmentID,
			"document_id":   documentID,
			"image_src":     src,
			"markdown":      "![" + alt + "](" + src + ")",
		},
	}
}
