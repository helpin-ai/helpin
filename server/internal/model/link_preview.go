package model

// LinkPreview is the complete allowlist of content published to link crawlers.
// It deliberately has no record DTO, message body, email, or actor identifiers.
type LinkPreview struct {
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Type        string             `json:"type"`
	Reference   string             `json:"reference,omitempty"`
	Fields      []LinkPreviewField `json:"fields,omitempty"`
	URL         string             `json:"url"`
	ImageURL    string             `json:"image_url"`
}

// LinkPreviewField is a labeled value drawn in the image, never the description.
type LinkPreviewField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// LinkPreviewRecord carries the projection and its module dependencies internally.
// It is never serialized by a handler.
type LinkPreviewRecord struct {
	Preview LinkPreview
	Modules []ModuleID
}
