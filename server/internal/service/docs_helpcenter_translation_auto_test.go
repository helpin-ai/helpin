package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// TestCollectAutoTranslateTargets covers the pure read-only phase of
// the auto-translate pipeline: given a workspace, a locale, and a mix
// of seeded spaces / collections / existing translations, we expect
// the helper to return only the rows that are still missing — in a
// deterministic order (space then its collections).
func TestCollectAutoTranslateTargets(t *testing.T) {
	ctx := context.Background()
	const (
		workspaceID = "ws-auto"
		userID      = "user-auto"
	)
	now := time.Now().UTC()
	pstr := func(s string) *string { return &s }

	t.Run("empty workspace returns empty slice", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("expected 0 targets, got %d", len(targets))
		}
	})

	t.Run("non-external-capable spaces are excluded", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-internal",
			WorkspaceID: workspaceID,
			Name:        "Internal Space",
			Slug:        "internal-space",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("internal spaces must not produce targets, got %d", len(targets))
		}
	})

	t.Run("deleted spaces are excluded", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		deletedAt := now
		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-deleted",
			WorkspaceID: workspaceID,
			Name:        "Deleted Space",
			Slug:        "deleted-space",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
			DeletedAt:   &deletedAt,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("deleted spaces must not produce targets, got %d", len(targets))
		}
	})

	t.Run("space with existing translation is not a target", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-filled",
			WorkspaceID: workspaceID,
			Name:        "Help Center",
			Slug:        "help-center",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
			ID:          "st-1",
			SpaceID:     "space-filled",
			WorkspaceID: workspaceID,
			Locale:      "fr",
			Name:        "Centre d'aide",
			Slug:        pstr("centre-daide"),
			Status:      model.DocsHelpcenterTranslationStatusDraft,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("space with existing translation must not be a target, got %d", len(targets))
		}
	})

	t.Run("space missing translation becomes a space target", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-bare",
			WorkspaceID: workspaceID,
			Name:        "Help Center",
			Slug:        "help-center",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}
		got := targets[0]
		if got.Kind != "space" {
			t.Errorf("Kind = %q, want %q", got.Kind, "space")
		}
		if got.EntityID != "space-bare" {
			t.Errorf("EntityID = %q, want %q", got.EntityID, "space-bare")
		}
		if got.SpaceID != "space-bare" {
			t.Errorf("SpaceID = %q, want %q", got.SpaceID, "space-bare")
		}
		if got.SourceName != "Help Center" {
			t.Errorf("SourceName = %q, want %q", got.SourceName, "Help Center")
		}
		if got.SourceDescription != "" {
			t.Errorf("SourceDescription must be empty for spaces, got %q", got.SourceDescription)
		}
	})

	t.Run("collection without description yields empty SourceDescription", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-cd",
			WorkspaceID: workspaceID,
			Name:        "Help Center",
			Slug:        "help-center",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
			ID:          "st-cd",
			SpaceID:     "space-cd",
			WorkspaceID: workspaceID,
			Locale:      "fr",
			Name:        "Centre d'aide",
			Slug:        pstr("centre-daide"),
			Status:      model.DocsHelpcenterTranslationStatusDraft,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-nodesc",
			SpaceID:     "space-cd",
			WorkspaceID: workspaceID,
			Name:        "Getting Started",
			Slug:        "getting-started",
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}
		got := targets[0]
		if got.Kind != "collection" {
			t.Errorf("Kind = %q, want %q", got.Kind, "collection")
		}
		if got.EntityID != "coll-nodesc" {
			t.Errorf("EntityID = %q", got.EntityID)
		}
		if got.SourceName != "Getting Started" {
			t.Errorf("SourceName = %q", got.SourceName)
		}
		if got.SourceDescription != "" {
			t.Errorf("SourceDescription = %q, want empty", got.SourceDescription)
		}
	})

	t.Run("collection with description carries it to the target", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-cwd",
			WorkspaceID: workspaceID,
			Name:        "Help Center",
			Slug:        "help-center",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
			ID:          "st-cwd",
			SpaceID:     "space-cwd",
			WorkspaceID: workspaceID,
			Locale:      "fr",
			Name:        "Centre d'aide",
			Slug:        pstr("centre-daide"),
			Status:      model.DocsHelpcenterTranslationStatusDraft,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-desc",
			SpaceID:     "space-cwd",
			WorkspaceID: workspaceID,
			Name:        "Getting Started",
			Slug:        "getting-started",
			Description: pstr("  Welcome to our product.  "), // whitespace is trimmed
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}
		if targets[0].SourceDescription != "Welcome to our product." {
			t.Errorf("SourceDescription = %q, want trimmed source text", targets[0].SourceDescription)
		}
	})

	t.Run("collection with existing translation is not a target", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-ct",
			WorkspaceID: workspaceID,
			Name:        "Help Center",
			Slug:        "help-center",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
			ID:          "st-ct",
			SpaceID:     "space-ct",
			WorkspaceID: workspaceID,
			Locale:      "fr",
			Name:        "Centre d'aide",
			Slug:        pstr("centre-daide"),
			Status:      model.DocsHelpcenterTranslationStatusDraft,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-done",
			SpaceID:     "space-ct",
			WorkspaceID: workspaceID,
			Name:        "Getting Started",
			Slug:        "getting-started",
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollectionTranslation(t, db, model.DocsHelpcenterCollectionTranslation{
			ID:           "ct-done",
			CollectionID: "coll-done",
			WorkspaceID:  workspaceID,
			SpaceID:      "space-ct",
			Locale:       "fr",
			Name:         "Commencer",
			Slug:         pstr("commencer"),
			Status:       model.DocsHelpcenterTranslationStatusDraft,
			CreatedAt:    now,
			UpdatedAt:    now,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("collection with existing translation must not be a target, got %d", len(targets))
		}
	})

	t.Run("deleted collections are excluded", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-dc",
			WorkspaceID: workspaceID,
			Name:        "Help Center",
			Slug:        "help-center",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
			ID:          "st-dc",
			SpaceID:     "space-dc",
			WorkspaceID: workspaceID,
			Locale:      "fr",
			Name:        "Centre d'aide",
			Slug:        pstr("centre-daide"),
			Status:      model.DocsHelpcenterTranslationStatusDraft,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		deletedAt := now
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-deleted",
			SpaceID:     "space-dc",
			WorkspaceID: workspaceID,
			Name:        "Archived",
			Slug:        "archived",
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
			DeletedAt:   &deletedAt,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("deleted collection must not be a target, got %d", len(targets))
		}
	})

	t.Run("ordering is space then its collections then next space", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		// Two external-capable spaces, each with one collection, all
		// missing translations. We expect the returned slice to be:
		//   space-A, coll-A, space-B, coll-B
		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-A",
			WorkspaceID: workspaceID,
			Name:        "Alpha",
			Slug:        "alpha",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-B",
			WorkspaceID: workspaceID,
			Name:        "Bravo",
			Slug:        "bravo",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			Position:    1,
			CreatedBy:   userID,
			CreatedAt:   now.Add(time.Second),
			UpdatedAt:   now.Add(time.Second),
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-A",
			SpaceID:     "space-A",
			WorkspaceID: workspaceID,
			Name:        "Alpha Collection",
			Slug:        "alpha-collection",
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-B",
			SpaceID:     "space-B",
			WorkspaceID: workspaceID,
			Name:        "Bravo Collection",
			Slug:        "bravo-collection",
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 4 {
			t.Fatalf("expected 4 targets, got %d", len(targets))
		}
		wantOrder := []struct {
			kind string
			id   string
		}{
			{"space", "space-A"},
			{"collection", "coll-A"},
			{"space", "space-B"},
			{"collection", "coll-B"},
		}
		for i, want := range wantOrder {
			if targets[i].Kind != want.kind || targets[i].EntityID != want.id {
				t.Errorf("targets[%d] = (%s, %s), want (%s, %s)",
					i, targets[i].Kind, targets[i].EntityID, want.kind, want.id)
			}
		}
	})

	t.Run("only the unfilled side of a partially-translated space is a target", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		// Space has a translation, collection does not. Expect only
		// the collection to be returned as a target.
		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-partial",
			WorkspaceID: workspaceID,
			Name:        "Partial",
			Slug:        "partial",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
			ID:          "st-partial",
			SpaceID:     "space-partial",
			WorkspaceID: workspaceID,
			Locale:      "fr",
			Name:        "Partiel",
			Slug:        pstr("partiel"),
			Status:      model.DocsHelpcenterTranslationStatusDraft,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-partial",
			SpaceID:     "space-partial",
			WorkspaceID: workspaceID,
			Name:        "Unfilled Collection",
			Slug:        "unfilled-collection",
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target (the collection), got %d", len(targets))
		}
		if targets[0].Kind != "collection" || targets[0].EntityID != "coll-partial" {
			t.Errorf("got (%s, %s), want (collection, coll-partial)", targets[0].Kind, targets[0].EntityID)
		}
	})

	t.Run("other-locale translations do not count for the requested locale", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		svc := newDocsHelpcenterTranslationServiceForTest(db)

		// Space has a German translation but no French one. Asking
		// for French should still return it as a target.
		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-de-only",
			WorkspaceID: workspaceID,
			Name:        "Multilingual",
			Slug:        "multilingual",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
			ID:          "st-de",
			SpaceID:     "space-de-only",
			WorkspaceID: workspaceID,
			Locale:      "de",
			Name:        "Hilfe",
			Slug:        pstr("hilfe"),
			Status:      model.DocsHelpcenterTranslationStatusDraft,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		targets, err := svc.collectAutoTranslateTargets(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}
		if targets[0].EntityID != "space-de-only" {
			t.Errorf("EntityID = %q, want %q", targets[0].EntityID, "space-de-only")
		}
	})
}
