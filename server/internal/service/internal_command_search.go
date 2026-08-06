package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	workspaceSearchDefaultLimit = 10
	workspaceSearchMaxLimit     = 50
	workspaceSearchMaxOffset    = 500
)

var workspaceSearchEntityTypes = []string{
	"task", "epic", "sprint", "objective", "document", "workspace_member",
	"crm_contact", "crm_company", "crm_deal", "support_conversation",
}

type workspaceSearchInput struct {
	Query       string   `json:"query"`
	EntityTypes []string `json:"entity_types"`
	Limit       int      `json:"limit"`
	Offset      int      `json:"offset"`
}

type workspaceSearchProvider interface {
	SearchLimit(ctx context.Context, workspaceID, query string, limit int) (*model.SearchResponse, error)
}

type workspaceSearchItem struct {
	EntityType   string   `json:"entity_type"`
	ID           string   `json:"id"`
	MarkdownLink string   `json:"markdown_link,omitempty"`
	Key          string   `json:"key,omitempty"`
	Title        string   `json:"title"`
	Context      string   `json:"context,omitempty"`
	MatchedOn    []string `json:"matched_on,omitempty"`
	score        float64
}

func (s *InternalCommandService) registerWorkspaceSearchCommands() {
	if s == nil {
		return
	}
	s.register(InternalCommandDefinition{
		Name:                   "workspace.search",
		Module:                 "workspace",
		Mutating:               false,
		SupportedTargetTypes:   []string{"workspace", "task", "epic", "sprint", "objective", "document", "crm_deal", "crm_contact", "support_conversation"},
		RequiredPermissionsAll: []authorization.Permission{authorization.PermSearchRead},
		Tool:                   mustCommandToolMetadata("workspace.search"),
		Execute:                s.executeSearchWorkspace,
	})
}

func (s *InternalCommandService) executeSearchWorkspace(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req workspaceSearchInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse search workspace input: %w", err)
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if len(req.Query) > 500 {
		return nil, fmt.Errorf("query must be 500 characters or fewer")
	}
	if req.Limit == 0 {
		req.Limit = workspaceSearchDefaultLimit
	}
	if req.Limit < 1 || req.Limit > workspaceSearchMaxLimit {
		return nil, fmt.Errorf("limit must be between 1 and 50")
	}
	if req.Offset < 0 || req.Offset > workspaceSearchMaxOffset {
		return nil, fmt.Errorf("offset must be between 0 and 500")
	}

	requested, err := normalizeWorkspaceSearchTypes(req.EntityTypes)
	if err != nil {
		return nil, err
	}
	allowed := s.allowedWorkspaceSearchTypes(meta)
	selected := requested
	if len(selected) == 0 {
		selected = orderedAllowedWorkspaceSearchTypes(allowed)
	} else {
		for _, entityType := range selected {
			if !allowed[entityType] {
				return nil, fmt.Errorf("entity_types contains inaccessible type %q", entityType)
			}
		}
	}

	fetchLimit := req.Offset + req.Limit + 1
	items, err := s.collectWorkspaceSearchItems(ctx, meta, req.Query, selected, fetchLimit)
	if err != nil {
		return nil, err
	}
	rankWorkspaceSearchItems(items, req.Query)
	total := len(items)
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + req.Limit
	if end > total {
		end = total
	}
	page := items[start:end]
	hasMore := end < total
	var nextOffset *int
	if hasMore {
		next := end
		nextOffset = &next
	}
	return mustJSON(map[string]any{
		"results":               page,
		"total":                 total,
		"limit":                 req.Limit,
		"offset":                req.Offset,
		"has_more":              hasMore,
		"next_offset":           nextOffset,
		"searched_entity_types": selected,
	}), nil
}

func normalizeWorkspaceSearchTypes(values []string) ([]string, error) {
	valid := make(map[string]bool, len(workspaceSearchEntityTypes))
	for _, value := range workspaceSearchEntityTypes {
		valid[value] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if !valid[value] {
			return nil, fmt.Errorf("entity_types contains unsupported type %q", value)
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out, nil
}

func (s *InternalCommandService) allowedWorkspaceSearchTypes(meta model.InternalCommandContext) map[string]bool {
	allowed := map[string]bool{
		"task": true, "epic": true, "sprint": true, "objective": true,
		"document": true, "workspace_member": true,
	}
	if s.authz == nil || strings.TrimSpace(meta.ActorRole) == "" {
		return allowed
	}
	actor := internalCommandActor(meta)
	if s.authz.Can(actor, authorization.PermCRMRead) {
		allowed["crm_contact"], allowed["crm_company"], allowed["crm_deal"] = true, true, true
	}
	if s.authz.Can(actor, authorization.PermSupportRead) {
		allowed["support_conversation"] = true
	}
	return allowed
}

func orderedAllowedWorkspaceSearchTypes(allowed map[string]bool) []string {
	out := make([]string, 0, len(allowed))
	for _, entityType := range workspaceSearchEntityTypes {
		if allowed[entityType] {
			out = append(out, entityType)
		}
	}
	return out
}

func workspaceSearchTypeSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func (s *InternalCommandService) collectWorkspaceSearchItems(ctx context.Context, meta model.InternalCommandContext, query string, selected []string, fetchLimit int) ([]workspaceSearchItem, error) {
	types := workspaceSearchTypeSet(selected)
	agentTeams, err := commandAgentTeamFilter(meta)
	if err != nil {
		return nil, err
	}
	items := make([]workspaceSearchItem, 0, fetchLimit)
	if s.workspaceSearch != nil && (types["task"] || types["epic"] || types["sprint"] || types["objective"] || types["document"] || types["workspace_member"]) {
		result, err := s.workspaceSearch.SearchLimit(ctx, meta.WorkspaceID, query, fetchLimit)
		if err != nil {
			return nil, fmt.Errorf("search workspace entities: %w", err)
		}
		appendCore := func(entityType string, values []model.SearchResult) {
			if !types[entityType] {
				return
			}
			for _, value := range values {
				if (entityType == "task" || entityType == "epic" || entityType == "sprint") && strings.TrimSpace(value.TeamID) != "" {
					teamID := strings.TrimSpace(value.TeamID)
					if !canAccessTeam(ctx, &teamID) || (len(agentTeams) > 0 && !containsCommandTeam(agentTeams, &teamID)) {
						continue
					}
				}
				contextText := value.TeamName
				label := value.Name
				if entityType == "task" && strings.TrimSpace(value.TaskKey) != "" {
					label = value.TaskKey
				}
				items = append(items, workspaceSearchItem{EntityType: entityType, ID: value.ID, MarkdownLink: helpinMarkdownLinkForEntityType(label, entityType, value.ID), Key: value.TaskKey, Title: value.Name, Context: contextText, MatchedOn: []string{"search_text"}})
			}
		}
		appendCore("task", result.Tasks)
		appendCore("epic", result.Epics)
		appendCore("sprint", result.Sprints)
		appendCore("objective", result.Objectives)
		appendCore("document", result.Documents)
		appendCore("workspace_member", result.Members)
	}
	if s.crmSearch != nil && (types["crm_contact"] || types["crm_company"] || types["crm_deal"]) {
		results, err := s.crmSearch.SearchLimit(ctx, meta.WorkspaceID, query, fetchLimit)
		if err != nil {
			return nil, fmt.Errorf("search CRM entities: %w", err)
		}
		for _, result := range results {
			entityType := "crm_" + strings.TrimSpace(result.Type)
			if types[entityType] {
				items = append(items, workspaceSearchItem{EntityType: entityType, ID: result.ID, MarkdownLink: helpinMarkdownLinkForEntityType(result.Name, entityType, result.ID), Title: result.Name, Context: result.Detail})
			}
		}
	}
	if types["support_conversation"] {
		supportItems, err := s.searchWorkspaceSupport(ctx, meta, query, fetchLimit)
		if err != nil {
			return nil, err
		}
		items = append(items, supportItems...)
	}
	return dedupeWorkspaceSearchItems(items), nil
}

func (s *InternalCommandService) searchWorkspaceSupport(ctx context.Context, meta model.InternalCommandContext, query string, fetchLimit int) ([]workspaceSearchItem, error) {
	if fetchLimit <= 0 {
		return nil, nil
	}
	memberID := ""
	if s.workspaceRepo != nil && strings.TrimSpace(meta.ActorID) != "" {
		if member, lookupErr := s.workspaceRepo.GetMembership(ctx, meta.WorkspaceID, meta.ActorID); lookupErr == nil && member != nil {
			memberID = member.ID
		}
	}
	results := make([]model.SupportConversationSearchResult, 0, fetchLimit)
	for batchOffset := 0; batchOffset < fetchLimit; {
		batchSize := min(50, fetchLimit-batchOffset)
		offset := batchOffset
		params := SupportConversationSearchParams{
			WorkspaceID: meta.WorkspaceID,
			UserID:      meta.ActorID,
			Query:       query,
			Pagination:  model.PMPagination{Page: 1, PerPage: batchSize, Offset: &offset},
		}
		var (
			response *model.SupportConversationSearchResponse
			err      error
		)
		if s.supportInboxService != nil {
			response, err = s.supportInboxService.SearchConversations(ctx, params)
		} else if s.supportConversationRepo != nil {
			response, err = s.supportConversationRepo.Search(ctx, repository.ConversationRepositorySearchParams{
				SupportConversationSearchParams: params,
				WorkspaceMemberID:               memberID,
				Role:                            meta.ActorRole,
			})
		}
		if err != nil {
			return nil, fmt.Errorf("search support conversations: %w", err)
		}
		if response == nil || len(response.Data) == 0 {
			break
		}
		results = append(results, response.Data...)
		batchOffset += len(response.Data)
		if len(response.Data) < batchSize || batchOffset >= response.Total {
			break
		}
	}
	items := make([]workspaceSearchItem, 0, len(results))
	for _, result := range results {
		conversation := result.Conversation
		title := strings.TrimSpace(conversation.Subject)
		if title == "" {
			title = strings.TrimSpace(derefString(conversation.CustomerName))
		}
		if title == "" {
			title = strings.TrimSpace(derefString(conversation.CustomerEmail))
		}
		key := ""
		if result.DisplayID > 0 {
			key = fmt.Sprintf("%d", result.DisplayID)
		}
		items = append(items, workspaceSearchItem{
			EntityType: "support_conversation", ID: conversation.ID, MarkdownLink: helpinMarkdownLink(title, "support-conversations", conversation.ID), Key: key,
			Title: title, Context: strings.TrimSpace(result.Snippet), MatchedOn: result.MatchedFields, score: result.Score,
		})
	}
	return items, nil
}

func dedupeWorkspaceSearchItems(items []workspaceSearchItem) []workspaceSearchItem {
	seen := map[string]bool{}
	out := make([]workspaceSearchItem, 0, len(items))
	for _, item := range items {
		key := item.EntityType + ":" + item.ID
		if item.ID == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func rankWorkspaceSearchItems(items []workspaceSearchItem, query string) {
	normalizedQuery := normalizeSearchText(query)
	for index := range items {
		item := &items[index]
		matched := append([]string(nil), item.MatchedOn...)
		normalizedTitle := normalizeSearchText(item.Title)
		normalizedKey := normalizeSearchText(item.Key)
		normalizedContext := normalizeSearchText(item.Context)
		switch {
		case normalizedQuery != "" && (normalizedQuery == normalizeSearchText(item.ID) || normalizedQuery == normalizedKey):
			item.score += 1000
			matched = append(matched, "exact_key")
		case normalizedQuery != "" && normalizedQuery == normalizedTitle:
			item.score += 900
			matched = append(matched, "exact_title")
		case normalizedQuery != "" && strings.HasPrefix(normalizedTitle, normalizedQuery):
			item.score += 700
			matched = append(matched, "title_prefix")
		case normalizedQuery != "" && strings.Contains(normalizedTitle, normalizedQuery):
			item.score += 500
			matched = append(matched, "title")
		case normalizedQuery != "" && strings.Contains(normalizedContext, normalizedQuery):
			item.score += 300
			matched = append(matched, "context")
		}
		item.MatchedOn = uniqueNonEmptySearchStrings(matched)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			if items[i].EntityType == items[j].EntityType {
				return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
			}
			return items[i].EntityType < items[j].EntityType
		}
		return items[i].score > items[j].score
	})
}

func uniqueNonEmptySearchStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
