package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

const maxWorkspaceSkillArchiveSize int64 = 10 * 1024 * 1024

var (
	ErrWorkspaceSkillNotFound           = errors.New("workspace skill not found")
	ErrWorkspaceSkillStorageUnavailable = errors.New("workspace skill storage is not configured")
)

type skillPackageStore interface {
	PutObject(ctx context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error
	DeleteObject(ctx context.Context, key string) error
}

var workspaceSkillKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)

func (s *AgentService) SetWorkspaceSkillStore(repo *repository.WorkspaceSkillRepository, store skillPackageStore) *AgentService {
	s.workspaceSkillRepo = repo
	s.skillPackageStore = store
	return s
}

func (s *AgentService) ListSkillCatalog(ctx context.Context, workspaceID string) (model.SkillCatalogResponse, error) {
	catalog := worker.ListSkillCatalog()
	if s.workspaceSkillRepo == nil || strings.TrimSpace(workspaceID) == "" {
		return catalog, nil
	}
	workspaceSkills, err := s.workspaceSkillRepo.ListByWorkspace(ctx, workspaceID, false)
	if err != nil {
		return model.SkillCatalogResponse{}, err
	}
	for _, skill := range workspaceSkills {
		skillID := skill.ID
		catalog.Skills = append(catalog.Skills, model.SkillCatalogEntry{
			ID:                &skillID,
			Key:               skill.Key,
			VersionKey:        skill.VersionKey,
			Title:             skill.Title,
			Description:       stringOrDefault(skill.Description, ""),
			SourceKind:        skill.SourceKind,
			SourceRuntime:     trimPtr(skill.SourceRuntime),
			RequiredTools:     parseJSONStringSlice(json.RawMessage(skill.RequiredTools)),
			SupportedRuntimes: parseJSONStringSlice(json.RawMessage(skill.SupportedRuntimes)),
		})
	}
	sort.Slice(catalog.Skills, func(i, j int) bool {
		if catalog.Skills[i].Key == catalog.Skills[j].Key {
			return catalog.Skills[i].SourceKind < catalog.Skills[j].SourceKind
		}
		return catalog.Skills[i].Key < catalog.Skills[j].Key
	})
	return catalog, nil
}

func (s *AgentService) CreateWorkspaceSkill(ctx context.Context, workspaceID, actorID string, req model.CreateWorkspaceSkillRequest) (*model.WorkspaceSkillResponse, error) {
	if s.workspaceSkillRepo == nil || s.skillPackageStore == nil {
		return nil, ErrWorkspaceSkillStorageUnavailable
	}
	key, err := normalizeWorkspaceSkillKey(req.Key)
	if err != nil {
		return nil, err
	}
	if existing, err := s.workspaceSkillRepo.GetActiveByKey(ctx, workspaceID, key); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, fmt.Errorf("a workspace skill with key %q already exists", key)
	}
	def, title, description, instructions := workspaceSkillDefinitionFromCreateRequest(req, key)
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if instructions == "" {
		return nil, fmt.Errorf("instructions are required")
	}
	archive, checksum, filename, err := worker.BuildSkillArchive(def)
	if err != nil {
		return nil, err
	}
	return s.storeWorkspaceSkillArchive(ctx, workspaceID, actorID, model.WorkspaceSkillSourceWorkspace, trimPtr(req.SourceRuntime), title, description, instructions, def, archive, checksum, filename)
}

func (s *AgentService) ImportWorkspaceSkill(ctx context.Context, workspaceID, actorID, archiveName string, archive io.Reader, size int64, sourceRuntime *string) (*model.WorkspaceSkillResponse, error) {
	if s.workspaceSkillRepo == nil || s.skillPackageStore == nil {
		return nil, ErrWorkspaceSkillStorageUnavailable
	}
	if strings.TrimSpace(archiveName) == "" {
		archiveName = "skill.zip"
	}
	data, err := io.ReadAll(io.LimitReader(archive, maxWorkspaceSkillArchiveSize+1))
	if err != nil {
		return nil, fmt.Errorf("read skill archive: %w", err)
	}
	if int64(len(data)) > maxWorkspaceSkillArchiveSize {
		return nil, fmt.Errorf("skill archive exceeds maximum size of 10MB")
	}
	def, err := worker.LoadSkillArchive(data, model.WorkspaceSkillSourceImported)
	if err != nil {
		return nil, err
	}
	key, err := normalizeWorkspaceSkillKey(def.Key)
	if err != nil {
		return nil, err
	}
	if existing, err := s.workspaceSkillRepo.GetActiveByKey(ctx, workspaceID, key); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, fmt.Errorf("a workspace skill with key %q already exists", key)
	}
	def.Key = key
	return s.storeWorkspaceSkillArchive(ctx, workspaceID, actorID, model.WorkspaceSkillSourceImported, trimPtr(sourceRuntime), strings.TrimSpace(def.Title), strings.TrimSpace(def.Description), strings.TrimSpace(def.Instructions), def, data, worker.SkillVersionForBytes(data), path.Base(archiveName))
}

func (s *AgentService) UpdateWorkspaceSkill(ctx context.Context, workspaceID, skillID string, req model.UpdateWorkspaceSkillRequest) (*model.WorkspaceSkillResponse, error) {
	if s.workspaceSkillRepo == nil || s.skillPackageStore == nil {
		return nil, ErrWorkspaceSkillStorageUnavailable
	}
	skill, err := s.workspaceSkillRepo.GetByID(ctx, workspaceID, skillID)
	if err != nil {
		return nil, err
	}
	if skill == nil || skill.IsArchived {
		return nil, ErrWorkspaceSkillNotFound
	}
	if skill.SourceKind != model.WorkspaceSkillSourceWorkspace {
		return nil, fmt.Errorf("imported skills are read-only; re-import a new package to change them")
	}
	key := skill.Key
	if req.Key != nil {
		key, err = normalizeWorkspaceSkillKey(*req.Key)
		if err != nil {
			return nil, err
		}
		if key != skill.Key {
			if existing, err := s.workspaceSkillRepo.GetActiveByKey(ctx, workspaceID, key); err != nil {
				return nil, err
			} else if existing != nil && existing.ID != skill.ID {
				return nil, fmt.Errorf("a workspace skill with key %q already exists", key)
			}
		}
	}
	title := skill.Title
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		title = strings.TrimSpace(*req.Title)
	}
	description := strings.TrimSpace(stringOrDefault(skill.Description, ""))
	if req.Description != nil {
		description = strings.TrimSpace(*req.Description)
	}
	instructions := strings.TrimSpace(skill.Instructions)
	if req.Instructions != nil {
		instructions = strings.TrimSpace(*req.Instructions)
	}
	requiredTools := parseJSONStringSlice(json.RawMessage(skill.RequiredTools))
	if req.RequiredTools != nil {
		requiredTools = worker.SortedUniqueStrings(req.RequiredTools)
	}
	supportedRuntimes := parseJSONStringSlice(json.RawMessage(skill.SupportedRuntimes))
	if req.SupportedRuntimes != nil {
		supportedRuntimes = worker.SortedUniqueStrings(req.SupportedRuntimes)
	}
	if strings.TrimSpace(description) == "" {
		return nil, fmt.Errorf("description is required")
	}
	if strings.TrimSpace(instructions) == "" {
		return nil, fmt.Errorf("instructions are required")
	}
	def := worker.SkillDefinition{Key: key, Title: title, Description: description, Instructions: instructions, RequiredTools: requiredTools, SupportedRuntimes: supportedRuntimes, SourceKind: model.WorkspaceSkillSourceWorkspace}
	archive, checksum, filename, err := worker.BuildSkillArchive(def)
	if err != nil {
		return nil, err
	}
	objectKey := fmt.Sprintf("workspaces/%s/skills/%s/%s", workspaceID, skill.ID, filename)
	if err := s.skillPackageStore.PutObject(ctx, objectKey, "application/zip", int64(len(archive)), bytes.NewReader(archive), false); err != nil {
		return nil, fmt.Errorf("store skill archive: %w", err)
	}
	if skill.PackageObjectKey != "" && skill.PackageObjectKey != objectKey {
		_ = s.skillPackageStore.DeleteObject(ctx, skill.PackageObjectKey)
	}
	skill.Key = key
	skill.Title = title
	skill.Description = trimPtr(&description)
	skill.Instructions = instructions
	skill.RequiredTools = marshalJSONBlob(worker.SortedUniqueStrings(requiredTools), []byte("[]"))
	skill.SupportedRuntimes = marshalJSONBlob(worker.SortedUniqueStrings(supportedRuntimes), []byte("[]"))
	if req.SourceRuntime != nil {
		skill.SourceRuntime = trimPtr(req.SourceRuntime)
	}
	skill.VersionKey = worker.SkillVersionForBytes(archive)
	skill.PackageObjectKey = objectKey
	skill.PackageFileName = filename
	skill.PackageSize = int64(len(archive))
	skill.PackageChecksum = checksum
	if err := s.workspaceSkillRepo.Update(ctx, skill); err != nil {
		return nil, err
	}
	resp := workspaceSkillResponse(skill)
	return &resp, nil
}

func (s *AgentService) DeleteWorkspaceSkill(ctx context.Context, workspaceID, skillID string) error {
	if s.workspaceSkillRepo == nil {
		return ErrWorkspaceSkillStorageUnavailable
	}
	skill, err := s.workspaceSkillRepo.GetByID(ctx, workspaceID, skillID)
	if err != nil {
		return err
	}
	if skill == nil || skill.IsArchived {
		return ErrWorkspaceSkillNotFound
	}
	// Keep the archive package in object storage when archiving so the original
	// skill payload remains available for audit/history and future restore flows.
	return s.workspaceSkillRepo.Archive(ctx, workspaceID, skillID)
}

func (s *AgentService) storeWorkspaceSkillArchive(ctx context.Context, workspaceID, actorID, sourceKind string, sourceRuntime *string, title, description, instructions string, def worker.SkillDefinition, archive []byte, checksum, filename string) (*model.WorkspaceSkillResponse, error) {
	skillID := uuid.NewString()
	objectKey := fmt.Sprintf("workspaces/%s/skills/%s/%s", workspaceID, skillID, path.Base(filename))
	if err := s.skillPackageStore.PutObject(ctx, objectKey, "application/zip", int64(len(archive)), bytes.NewReader(archive), false); err != nil {
		return nil, fmt.Errorf("store skill archive: %w", err)
	}
	skill := &model.WorkspaceSkill{
		ID:                skillID,
		WorkspaceID:       workspaceID,
		SourceKind:        sourceKind,
		SourceRuntime:     sourceRuntime,
		Key:               def.Key,
		VersionKey:        worker.SkillVersionForBytes(archive),
		Title:             title,
		Description:       trimPtr(&description),
		Instructions:      instructions,
		RequiredTools:     marshalJSONBlob(worker.SortedUniqueStrings(def.RequiredTools), []byte("[]")),
		SupportedRuntimes: marshalJSONBlob(worker.SortedUniqueStrings(def.SupportedRuntimes), []byte("[]")),
		InterfaceConfig:   marshalJSONBlob(def.Interface, []byte("{}")),
		PolicyConfig:      marshalJSONBlob(def.Policy, []byte("{}")),
		PackageObjectKey:  objectKey,
		PackageFileName:   path.Base(filename),
		PackageSize:       int64(len(archive)),
		PackageChecksum:   checksum,
		CreatedBy:         trimPtr(&actorID),
	}
	if err := s.workspaceSkillRepo.Create(ctx, skill); err != nil {
		return nil, err
	}
	resp := workspaceSkillResponse(skill)
	return &resp, nil
}

func workspaceSkillDefinitionFromCreateRequest(req model.CreateWorkspaceSkillRequest, key string) (worker.SkillDefinition, string, string, string) {
	description := strings.TrimSpace(req.Description)
	instructions := strings.TrimSpace(req.Instructions)
	title := strings.TrimSpace(stringOrDefault(req.Title, ""))
	if title == "" {
		title = key
	}
	def := worker.SkillDefinition{Key: key, Title: title, Description: description, Instructions: instructions, RequiredTools: worker.SortedUniqueStrings(req.RequiredTools), SupportedRuntimes: worker.SortedUniqueStrings(req.SupportedRuntimes), SourceKind: model.WorkspaceSkillSourceWorkspace}
	return def, title, description, instructions
}

func workspaceSkillResponse(skill *model.WorkspaceSkill) model.WorkspaceSkillResponse {
	return model.WorkspaceSkillResponse{ID: skill.ID, WorkspaceID: skill.WorkspaceID, SourceKind: skill.SourceKind, SourceRuntime: trimPtr(skill.SourceRuntime), Key: skill.Key, VersionKey: skill.VersionKey, Title: skill.Title, Description: stringOrDefault(skill.Description, ""), Instructions: skill.Instructions, RequiredTools: parseJSONStringSlice(json.RawMessage(skill.RequiredTools)), SupportedRuntimes: parseJSONStringSlice(json.RawMessage(skill.SupportedRuntimes)), PackageFileName: skill.PackageFileName, PackageSize: skill.PackageSize, IsArchived: skill.IsArchived, CreatedBy: trimPtr(skill.CreatedBy), CreatedAt: skill.CreatedAt, UpdatedAt: skill.UpdatedAt}
}

func normalizeWorkspaceSkillKey(raw string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(raw, " ", "_"), "-", "_")))
	if key == "" {
		return "", fmt.Errorf("skill key is required")
	}
	if !workspaceSkillKeyPattern.MatchString(key) {
		return "", fmt.Errorf("skill key must use lowercase letters, numbers, or underscores")
	}
	return key, nil
}

func marshalJSONBlob(value any, fallback []byte) model.JSONBlob {
	payload, err := json.Marshal(value)
	if err != nil {
		return model.JSONBlob(fallback)
	}
	return model.JSONBlob(payload)
}
