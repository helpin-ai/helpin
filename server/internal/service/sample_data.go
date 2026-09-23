package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SampleDataSeeder seeds the sample records of one product module. Seeders run
// inside the load transaction and must create every row through env.Tx.
type SampleDataSeeder interface {
	Module() model.ModuleID
	Seed(ctx context.Context, env *SampleDataEnv) error
}

// SampleDataEnv carries the load transaction and cross-module references.
type SampleDataEnv struct {
	Tx          *gorm.DB
	WorkspaceID string
	ActorID     string
	// ActorMemberID is the actor's workspace member ID.
	ActorMemberID string
	// ActorName is the display name used on sample teammate replies.
	ActorName string
	Now       time.Time
	Repo      *repository.SampleDataRepository

	// Cross-module references keyed by fixture key, filled by earlier seeders.
	ContactIDs map[string]string
	CompanyIDs map[string]string
	TaskIDs    map[string]string

	items []model.SampleDataItem
}

// Track records a created entity as sample data.
func (e *SampleDataEnv) Track(entityType, entityID string) {
	item := model.SampleDataItem{WorkspaceID: e.WorkspaceID, EntityType: entityType, EntityID: entityID}
	if e.ActorID != "" {
		actorID := e.ActorID
		item.CreatedBy = &actorID
	}
	e.items = append(e.items, item)
}

// SampleDataService loads and removes the sample workspace content.
//
// Loading runs in one database transaction. Seeders build module services on
// that transaction without websocket publishers, notification, automation,
// triage, translation, summary, or analytics dependencies, so business rules
// (validation, numbering, positions, activity history) apply while no email,
// notification, webhook, automation rule, or AI call is triggered.
type SampleDataService struct {
	repo    *repository.SampleDataRepository
	seeders []SampleDataSeeder
	now     func() time.Time
	logger  *slog.Logger
}

// NewSampleDataService creates a SampleDataService with the built-in seeders.
func NewSampleDataService(db *gorm.DB, docsUseSortKey bool) *SampleDataService {
	return NewSampleDataServiceWithSeeders(db, DefaultSampleDataSeeders(docsUseSortKey)...)
}

// NewSampleDataServiceWithSeeders creates a SampleDataService with explicit seeders.
func NewSampleDataServiceWithSeeders(db *gorm.DB, seeders ...SampleDataSeeder) *SampleDataService {
	return &SampleDataService{
		repo:    repository.NewSampleDataRepository(db),
		seeders: seeders,
		now:     func() time.Time { return time.Now().UTC() },
		logger:  slog.Default().With("service", "sample_data"),
	}
}

// Status reports the sample data loaded in a workspace. enabled lists the
// modules the caller can use; it limits the modules reported as seedable.
func (s *SampleDataService) Status(ctx context.Context, workspaceID string, enabled map[model.ModuleID]bool) (*model.SampleDataStatus, error) {
	items, err := s.repo.ListItems(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.status(items, enabled), nil
}

// Load seeds sample data for every enabled module in one transaction. It
// returns model.ErrSampleDataAlreadyLoaded when the workspace already holds
// sample data and model.ErrSampleDataNoModules when nothing can be seeded.
func (s *SampleDataService) Load(ctx context.Context, workspaceID, actorID string, enabled map[model.ModuleID]bool) (*model.SampleDataStatus, error) {
	seeders := s.enabledSeeders(enabled)
	if len(seeders) == 0 {
		return nil, model.ErrSampleDataNoModules
	}
	var items []model.SampleDataItem
	err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		if err := repo.LockWorkspace(ctx, workspaceID); err != nil {
			return err
		}
		existing, err := repo.ListItems(ctx, workspaceID)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			return model.ErrSampleDataAlreadyLoaded
		}
		env, err := s.newEnv(ctx, tx, repo, workspaceID, actorID)
		if err != nil {
			return err
		}
		for _, seeder := range seeders {
			if err := seeder.Seed(ctx, env); err != nil {
				return fmt.Errorf("seed %s sample data: %w", seeder.Module(), err)
			}
		}
		if err := repo.Track(ctx, env.items); err != nil {
			return err
		}
		items, err = repo.ListItems(ctx, workspaceID)
		return err
	})
	if err != nil {
		if !errors.Is(err, model.ErrSampleDataAlreadyLoaded) {
			s.logger.ErrorContext(ctx, "sample data load failed", "error", err, "workspace_id", workspaceID, "user_id", actorID)
		}
		return nil, err
	}
	s.logger.InfoContext(ctx, "sample data loaded", "workspace_id", workspaceID, "user_id", actorID, "records", len(items))
	return s.status(items, enabled), nil
}

// Remove deletes every tracked sample record in dependency order in one
// transaction. Containers that users filled with their own records are kept
// and reported in Retained.
func (s *SampleDataService) Remove(ctx context.Context, workspaceID, actorID string, enabled map[model.ModuleID]bool) (*model.SampleDataStatus, error) {
	retained := map[string]int64{}
	removed := 0
	err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		if err := repo.LockWorkspace(ctx, workspaceID); err != nil {
			return err
		}
		items, err := repo.ListItems(ctx, workspaceID)
		if err != nil {
			return err
		}
		byType := map[string][]model.SampleDataItem{}
		for _, item := range items {
			byType[item.EntityType] = append(byType[item.EntityType], item)
		}
		for _, entityType := range model.SampleEntityRemovalOrder {
			for _, item := range byType[entityType] {
				err := repo.DeleteEntity(ctx, workspaceID, entityType, item.EntityID)
				if errors.Is(err, repository.ErrSampleEntityRetained) {
					retained[entityType]++
					continue
				}
				if err != nil {
					return err
				}
				removed++
			}
			delete(byType, entityType)
		}
		for entityType := range byType {
			return fmt.Errorf("unknown sample entity type %q", entityType)
		}
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		return repo.DeleteItems(ctx, workspaceID, ids)
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "sample data removal failed", "error", err, "workspace_id", workspaceID, "user_id", actorID)
		return nil, err
	}
	s.logger.InfoContext(ctx, "sample data removed", "workspace_id", workspaceID, "user_id", actorID, "records", removed)
	status := s.status(nil, enabled)
	if len(retained) > 0 {
		status.Retained = retained
	}
	return status, nil
}

func (s *SampleDataService) newEnv(ctx context.Context, tx *gorm.DB, repo *repository.SampleDataRepository, workspaceID, actorID string) (*SampleDataEnv, error) {
	member, err := repository.NewWorkspaceRepository(tx).GetMembership(ctx, workspaceID, actorID)
	if err != nil {
		return nil, fmt.Errorf("resolve sample data actor: %w", err)
	}
	if member == nil {
		return nil, &model.ErrForbidden{Message: "active workspace membership required"}
	}
	return &SampleDataEnv{
		Tx:            tx,
		WorkspaceID:   workspaceID,
		ActorID:       actorID,
		ActorMemberID: member.ID,
		ActorName:     member.DisplayName,
		Now:           s.now(),
		Repo:          repo,
		ContactIDs:    map[string]string{},
		CompanyIDs:    map[string]string{},
		TaskIDs:       map[string]string{},
	}, nil
}

// sampleDataCompanion marks a seeder that only adds examples around content
// seeded by other modules (sample Flows act on sample tasks). It runs only
// when at least one content seeder is enabled.
type sampleDataCompanion interface {
	sampleDataCompanion()
}

func (s *SampleDataService) enabledSeeders(enabled map[model.ModuleID]bool) []SampleDataSeeder {
	var result []SampleDataSeeder
	hasContent := false
	for _, seeder := range s.seeders {
		if !enabled[seeder.Module()] {
			continue
		}
		if _, companion := seeder.(sampleDataCompanion); !companion {
			hasContent = true
		}
		result = append(result, seeder)
	}
	if !hasContent {
		return nil
	}
	return result
}

func (s *SampleDataService) status(items []model.SampleDataItem, enabled map[model.ModuleID]bool) *model.SampleDataStatus {
	status := &model.SampleDataStatus{Loaded: len(items) > 0, Counts: map[string]int64{}, Modules: []model.ModuleID{}}
	for _, item := range items {
		status.Counts[item.EntityType]++
		if status.LoadedAt == nil || item.CreatedAt.Before(*status.LoadedAt) {
			createdAt := item.CreatedAt
			status.LoadedAt = &createdAt
		}
	}
	for _, seeder := range s.enabledSeeders(enabled) {
		status.Modules = append(status.Modules, seeder.Module())
	}
	return status
}
