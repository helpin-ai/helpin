package model

// DocsAIRewriteRequest rewrites caller-supplied selection HTML without saving a document.
type DocsAIRewriteRequest struct {
	Content      string `json:"content"`
	Operation    string `json:"operation"`
	Instructions string `json:"instructions,omitempty"`
}
type DocsAIRewriteResponse struct {
	Content string `json:"content"`
}
