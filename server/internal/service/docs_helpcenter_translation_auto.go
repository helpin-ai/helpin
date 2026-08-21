package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/llm"
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

// plainStringPointerOrNil returns a pointer to the trimmed value or
// nil when the trimmed value is empty. Unlike stringPointerOrNil in
// the wider translation package, this helper does NOT slugify the
// value — it preserves the original text verbatim (minus outer
// whitespace) and is safe to use for descriptions or any free-text
// field that must round-trip without being mangled into url-safe
// characters.
func plainStringPointerOrNil(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// AutoTranslateMissing fills in every missing translation row for
// the requested locale in one LLM call. It is the bulk version of
// GenerateSpaceTranslation / GenerateCollectionTranslation — instead
// of N HTTP requests hitting N LLM calls, the entire workspace is
// scanned once, the missing targets are batched into a single
// numbered prompt, the model returns one JSON array, and the
// existing UpsertSpaceTranslation / UpsertCollectionTranslation
// helpers write the rows.
//
// Safety invariants (see 2026-04-11 Option C plan):
//  1. Index-based mapping from prompt → response, never UUIDs. If
//     the model hallucinates or drops items we notice immediately
//     and mark the affected targets as failed.
//  2. Existing rows are never overwritten. Every write re-checks
//     for a translation row at write time to handle races with
//     single-cell generate / other admin sessions.
//  3. Writes delegate to the shared upsert methods so slug freeze,
//     uniqueness, and event publishing behave identically to every
//     other translation write path.
//  4. Partial failures are reported per-target, never fatal to the
//     rest of the batch. A malformed item at index 7 does not stop
//     items 1-6 or 8-N from being written.
//  5. When the target list is empty we return an empty result
//     without calling the LLM at all — saves cost and latency on
//     the common "already translated" case.
func (s *DocsHelpcenterTranslationService) AutoTranslateMissing(
	ctx context.Context,
	workspaceID, locale string,
) (*model.AutoTranslateMissingResponse, error) {
	cfg, err := s.hcRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("help center config not found")
	}
	normalizedLocale, err := validateEditableLocale(cfg, locale)
	if err != nil {
		return nil, err
	}

	targets, err := s.collectAutoTranslateTargets(ctx, workspaceID, normalizedLocale)
	if err != nil {
		return nil, err
	}

	result := &model.AutoTranslateMissingResponse{
		Locale:    normalizedLocale,
		Requested: len(targets),
	}

	// Zero-target short-circuit: nothing to do, don't spend LLM budget.
	if len(targets) == 0 {
		return result, nil
	}

	if s.llmProvider == nil {
		return nil, fmt.Errorf("llm provider not configured")
	}

	prompt := buildAutoTranslatePrompt(targets, normalizedLocale)
	resp, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureDocsArticleTranslation,
		IdempotencyKey: aiUsageIdempotencyKey(workspaceID, BillingFeatureDocsArticleTranslation, "auto_translate_missing", normalizedLocale, fmt.Sprintf("%d", len(targets)), aiUsageStableHash(prompt)),
		Metadata: map[string]interface{}{
			"locale":       normalizedLocale,
			"target_count": len(targets),
		},
		Chat: llm.ChatRequest{
			Messages: []llm.Message{
				{Role: "user", Content: prompt},
			},
			Temperature: 0.2,
			JSONMode:    true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("auto-translate llm call: %w", err)
	}

	byIndex, err := parseAutoTranslateResponse(resp.Content, len(targets))
	if err != nil {
		return nil, fmt.Errorf("auto-translate parse: %w", err)
	}

	for i := range targets {
		target := targets[i]
		item, ok := byIndex[i+1]
		if !ok {
			result.Failed = append(result.Failed, model.AutoTranslateFailedItem{
				Kind:   target.Kind,
				ID:     target.EntityID,
				Reason: "no llm output for this target",
			})
			continue
		}

		switch target.Kind {
		case "space":
			// Re-check existence at write time to handle races with
			// single-cell Generate / other admin tabs.
			existing, err := s.translationRepo.GetSpaceTranslation(ctx, target.EntityID, normalizedLocale)
			if err != nil {
				result.Failed = append(result.Failed, model.AutoTranslateFailedItem{
					Kind: target.Kind, ID: target.EntityID, Reason: err.Error(),
				})
				continue
			}
			if existing != nil {
				// Someone else already filled it — desired state reached, skip.
				continue
			}
			row, err := s.UpsertSpaceTranslation(ctx, target.EntityID, model.UpsertDocsHelpcenterSpaceTranslationRequest{
				Locale:      normalizedLocale,
				Name:        item.Name,
				Description: plainStringPointerOrNil(item.Description),
			})
			if err != nil {
				result.Failed = append(result.Failed, model.AutoTranslateFailedItem{
					Kind: target.Kind, ID: target.EntityID, Reason: err.Error(),
				})
				continue
			}
			if row != nil {
				result.Spaces = append(result.Spaces, *row)
			}

		case "collection":
			existing, err := s.translationRepo.GetCollectionTranslation(ctx, target.EntityID, normalizedLocale)
			if err != nil {
				result.Failed = append(result.Failed, model.AutoTranslateFailedItem{
					Kind: target.Kind, ID: target.EntityID, Reason: err.Error(),
				})
				continue
			}
			if existing != nil {
				continue
			}
			row, err := s.UpsertCollectionTranslation(ctx, target.EntityID, model.UpsertDocsHelpcenterCollectionTranslationRequest{
				Locale:      normalizedLocale,
				Name:        item.Name,
				Description: plainStringPointerOrNil(item.Description),
			})
			if err != nil {
				result.Failed = append(result.Failed, model.AutoTranslateFailedItem{
					Kind: target.Kind, ID: target.EntityID, Reason: err.Error(),
				})
				continue
			}
			if row != nil {
				result.Collections = append(result.Collections, *row)
			}
		}
	}

	return result, nil
}
