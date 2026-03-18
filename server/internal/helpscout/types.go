package helpscout

import "encoding/json"

// Collection represents a HelpScout Docs collection (top-level grouping).
type Collection struct {
	ID         string `json:"id"`
	Number     int    `json:"number"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
	Order      int    `json:"order"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

// Category represents a category within a HelpScout Docs collection.
type Category struct {
	ID           string `json:"id"`
	Number       int    `json:"number"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	CollectionID string `json:"collectionId"`
	Order        int    `json:"order"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// ArticleRef represents the summary fields of a HelpScout Docs article (no body).
type ArticleRef struct {
	ID           string   `json:"id"`
	Number       int      `json:"number"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	HasDraft     bool     `json:"hasDraft"`
	CollectionID string   `json:"collectionId"`
	Categories   []string `json:"categories"`
}

// Article represents a full HelpScout Docs article including the HTML body.
type Article struct {
	ArticleRef
	Text string `json:"text"`
}

// paginatedResponse holds the raw JSON items and pagination metadata from
// the HelpScout Docs API. The items key varies per endpoint (e.g.
// "collections", "categories", "articles"), so callers unmarshal Items
// separately.
type paginatedResponse struct {
	Items json.RawMessage `json:"-"`
	Page  int             `json:"page"`
	Pages int             `json:"pages"`
}
