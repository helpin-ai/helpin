// Package intercom reads Help Center content and conversations from the
// Intercom REST API for imports.
package intercom

// Pages describes page-number pagination.
type Pages struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
	Next       *struct {
		Page          int    `json:"page"`
		StartingAfter string `json:"starting_after"`
	} `json:"next"`
}

// Me is the admin the access token belongs to.
type Me struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	App   struct {
		Name   string `json:"name"`
		IDCode string `json:"id_code"`
	} `json:"app"`
}

// Collection is a Help Center collection.
type Collection struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	URL          string  `json:"url"`
	ParentID     *string `json:"parent_id"`
	HelpCenterID *int64  `json:"help_center_id"`
	Order        int     `json:"order"`
}

// Article is a Help Center article; Body is HTML.
type Article struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Body        string  `json:"body"`
	State       string  `json:"state"` // published or draft
	URL         string  `json:"url"`
	ParentID    *int64  `json:"parent_id"`
	ParentType  *string `json:"parent_type"` // collection
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

// Author is who wrote a conversation part.
type Author struct {
	Type  string `json:"type"` // user, lead, contact, admin, bot, team
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ConversationSource is the message that started a conversation.
type ConversationSource struct {
	Type        string `json:"type"` // conversation, email, push, ...
	ID          string `json:"id"`
	DeliveredAs string `json:"delivered_as"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	Author      Author `json:"author"`
}

// ConversationPart is one event in a conversation.
type ConversationPart struct {
	ID        string  `json:"id"`
	PartType  string  `json:"part_type"` // comment, note, assignment, close, open, ...
	Body      *string `json:"body"`
	CreatedAt int64   `json:"created_at"`
	Author    Author  `json:"author"`
}

// Conversation is a conversation with its parts.
type Conversation struct {
	ID        string             `json:"id"`
	Title     *string            `json:"title"`
	State     string             `json:"state"` // open, closed, snoozed
	Priority  string             `json:"priority"`
	CreatedAt int64              `json:"created_at"`
	UpdatedAt int64              `json:"updated_at"`
	Source    ConversationSource `json:"source"`
	Contacts  struct {
		Contacts []struct {
			ID string `json:"id"`
		} `json:"contacts"`
	} `json:"contacts"`
	Tags struct {
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
	} `json:"tags"`
	Statistics *struct {
		LastCloseAt *int64 `json:"last_close_at"`
	} `json:"statistics"`
	Parts struct {
		Parts      []ConversationPart `json:"conversation_parts"`
		TotalCount int                `json:"total_count"`
	} `json:"conversation_parts"`
}

// ConversationRef is a conversation in a search result.
type ConversationRef struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// Contact is a user or lead.
type Contact struct {
	ID    string  `json:"id"`
	Role  string  `json:"role"`
	Email *string `json:"email"`
	Name  *string `json:"name"`
}
