package model

// SearchResult represents a single search result item.
type SearchResult struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	DisplayID int    `json:"display_id,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
	TeamName  string `json:"team_name,omitempty"`
}

// SearchResponse groups search results by entity type.
type SearchResponse struct {
	Stories    []SearchResult `json:"stories"`
	Epics      []SearchResult `json:"epics"`
	Sprints    []SearchResult `json:"sprints"`
	Objectives []SearchResult `json:"objectives"`
	Members    []SearchResult `json:"members"`
}
