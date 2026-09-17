package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) registerDirectGitCommands() {
	s.register(InternalCommandDefinition{Name: "git.open_pr", Module: "git", Mutating: true, RequiredPermissionsAll: []authorization.Permission{authorization.PermPMEdit, authorization.PermIntegrationsEnumerate}, Tool: mustCommandToolMetadata("git.open_pr"), SupportedTargetTypes: []string{"workspace", "repository", "task", "epic"}, Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
		var args struct {
			RepositoryID string `json:"repository_id"`
			Head         string `json:"head"`
			Base         string `json:"base"`
			Title        string `json:"title"`
			Body         string `json:"body"`
		}
		decoder := json.NewDecoder(bytes.NewReader(input))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&args); err != nil {
			return nil, err
		}
		args.RepositoryID = strings.TrimSpace(args.RepositoryID)
		args.Head = strings.TrimSpace(args.Head)
		args.Base = strings.TrimSpace(args.Base)
		args.Title = strings.TrimSpace(args.Title)
		if args.RepositoryID == "" || args.Head == "" || args.Base == "" || args.Title == "" {
			return nil, fmt.Errorf("repository_id, head, base and title are required")
		}
		if args.Head == args.Base {
			return nil, fmt.Errorf("head must differ from base")
		}
		if s.gitService == nil {
			return nil, fmt.Errorf("git service is unavailable")
		}
		repo, err := s.gitService.repoRepo.GetEnabledByID(ctx, meta.WorkspaceID, args.RepositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository is not available")
		}
		integration, err := s.gitService.integrationRepo.GetByID(ctx, meta.WorkspaceID, repo.IntegrationID)
		if err != nil {
			return nil, err
		}
		if integration == nil || !integration.Active {
			return nil, fmt.Errorf("git integration is not available")
		}
		result, err := s.gitService.ensureDelegatedRunPullRequest(ctx, integration, repo, args.Head, args.Base, args.Title, args.Body)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("repository provider does not support pull requests")
		}
		return json.Marshal(map[string]any{"number": result.Number, "title": result.Title, "url": result.URL, "provider": result.Provider})
	}})
}
