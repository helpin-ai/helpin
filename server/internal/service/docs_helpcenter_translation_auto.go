package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// autoTranslateTarget describes a single space or collection that
// currently has no translation row for a given locale and is therefore
// a candidate for batch auto-translation. Targets are produced in a
// deterministic order (space before its collections, collections in
// the repository's natural order) so the numeric index used in the
// LLM prompt is stable for the lifetime of one collect+write cycle.
//
// Spaces have no description field at the model layer, so a space
// target's SourceDescription is always empty. The frontend hides
// description cells on space rows to reflect this.
type autoTranslateTarget struct {
	Kind              string // "space" | "collection"
	EntityID          string // docs_spaces.id or docs_collections.id
	SpaceID           string // for spaces, equals EntityID; for collections, the parent space id
	SourceName        string
	SourceDescription string
}

// collectAutoTranslateTargets scans the workspace for every
// external-capable space plus every collection within those spaces
// and returns the ones missing a translation row for the requested
// locale. Soft-deleted rows and non-external spaces are excluded.
//
// The function is read-only and makes no LLM calls — callers pass
// the returned slice to prompt-building / write phases separately.
// Ordering is: for each space (in repository order), the space itself
// (if missing) followed by its collections (if missing). This order
// is what the LLM prompt will echo back via 1-based indices, so
// downstream code can rely on it being stable across the whole
// auto-translate operation.
func (s *DocsHelpcenterTranslationService) collectAutoTranslateTargets(
	ctx context.Context,
	workspaceID, locale string,
) ([]autoTranslateTarget, error) {
	spaces, err := s.spaceRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace spaces: %w", err)
	}

	var targets []autoTranslateTarget
	for i := range spaces {
		space := spaces[i]
		if space.Type != model.SpaceTypeExternalCapable {
			continue
		}
		if space.DeletedAt != nil {
			continue
		}

		// Space-level target: missing translation row for this locale.
		spaceTranslation, err := s.translationRepo.GetSpaceTranslation(ctx, space.ID, locale)
		if err != nil {
			return nil, fmt.Errorf("get space translation: %w", err)
		}
		if spaceTranslation == nil {
			targets = append(targets, autoTranslateTarget{
				Kind:       "space",
				EntityID:   space.ID,
				SpaceID:    space.ID,
				SourceName: space.Name,
			})
		}

		// Collection-level targets for every collection in this space
		// that has no translation row for the locale.
		collections, err := s.collectionRepo.ListBySpace(ctx, space.ID)
		if err != nil {
			return nil, fmt.Errorf("list space collections: %w", err)
		}
		for j := range collections {
			collection := collections[j]
			if collection.DeletedAt != nil {
				continue
			}
			collectionTranslation, err := s.translationRepo.GetCollectionTranslation(ctx, collection.ID, locale)
			if err != nil {
				return nil, fmt.Errorf("get collection translation: %w", err)
			}
			if collectionTranslation != nil {
				continue
			}
			description := ""
			if collection.Description != nil {
				description = strings.TrimSpace(*collection.Description)
			}
			targets = append(targets, autoTranslateTarget{
				Kind:              "collection",
				EntityID:          collection.ID,
				SpaceID:           space.ID,
				SourceName:        collection.Name,
				SourceDescription: description,
			})
		}
	}

	return targets, nil
}
