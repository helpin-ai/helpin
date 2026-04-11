package service

import (
	"context"
	"errors"
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

// TestAutoTranslateMissing exercises the end-to-end service method:
// collect targets → LLM call → parse → write rows via the existing
// upsert helpers. A scripted LLM returns a canned JSON array so the
// test runs without any real network traffic.
func TestAutoTranslateMissing(t *testing.T) {
	ctx := context.Background()
	const workspaceID = "ws-auto-e2e"
	const userID = "user-auto-e2e"
	now := time.Now().UTC()
	pstr := func(s string) *string { return &s }

	// Helper that stitches together the shared setup: tables,
	// helpcenter config with French enabled, one external space,
	// two collections (one with description, one without), and a
	// service instance bound to a scripted LLM.
	setup := func(t *testing.T, llmResponse string, llmErr error) (*DocsHelpcenterTranslationService, *scriptedDocsTranslationLLM, string, string, string) {
		t.Helper()
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		provider := &scriptedDocsTranslationLLM{response: llmResponse, err: llmErr}
		svc := newDocsHelpcenterTranslationServiceForTestWithLLM(db, provider)

		seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
			ID:                      "cfg-auto",
			WorkspaceID:             workspaceID,
			Subdomain:               "auto",
			BrandName:               "Auto",
			BrandColor:              "#000000",
			ThemeMode:               "light",
			DefaultLocale:           "en",
			EnabledLocales:          model.DocsStringArray{"en", "fr"},
			FallbackToDefaultLocale: true,
			CreatedAt:               now,
			UpdatedAt:               now,
		})
		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-auto",
			WorkspaceID: workspaceID,
			Name:        "Help Center",
			Slug:        "help-center",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-desc",
			SpaceID:     "space-auto",
			WorkspaceID: workspaceID,
			Name:        "Getting Started",
			Slug:        "getting-started",
			Description: pstr("Welcome to our product."),
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-nodesc",
			SpaceID:     "space-auto",
			WorkspaceID: workspaceID,
			Name:        "Billing",
			Slug:        "billing",
			Position:    1,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		return svc, provider, "space-auto", "coll-desc", "coll-nodesc"
	}

	t.Run("happy path: all targets translated and written", func(t *testing.T) {
		// The fixture has 3 missing targets in deterministic order:
		//   index 1 = space-auto (Help Center)
		//   index 2 = coll-desc (Getting Started)
		//   index 3 = coll-nodesc (Billing)
		// The LLM returns a matching JSON array.
		llmResponse := `[
			{"index": 1, "name": "Centre d'aide", "description": ""},
			{"index": 2, "name": "Commencer", "description": "Bienvenue dans notre produit."},
			{"index": 3, "name": "Facturation", "description": ""}
		]`
		svc, provider, spaceID, collDescID, collNoDescID := setup(t, llmResponse, nil)

		result, err := svc.AutoTranslateMissing(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("AutoTranslateMissing: %v", err)
		}
		if result.Locale != "fr" {
			t.Errorf("Locale = %q, want fr", result.Locale)
		}
		if result.Requested != 3 {
			t.Errorf("Requested = %d, want 3", result.Requested)
		}
		if len(result.Spaces) != 1 {
			t.Errorf("Spaces len = %d, want 1", len(result.Spaces))
		}
		if len(result.Collections) != 2 {
			t.Errorf("Collections len = %d, want 2", len(result.Collections))
		}
		if len(result.Failed) != 0 {
			t.Errorf("Failed len = %d, want 0: %+v", len(result.Failed), result.Failed)
		}
		if len(provider.requests) != 1 {
			t.Errorf("expected exactly 1 llm call, got %d", len(provider.requests))
		}

		// Verify correct mapping: index 1 → space, index 2 → coll-desc, index 3 → coll-nodesc
		if result.Spaces[0].SpaceID != spaceID {
			t.Errorf("space translation wrong space, got %q", result.Spaces[0].SpaceID)
		}
		if result.Spaces[0].Name != "Centre d'aide" {
			t.Errorf("space translation name = %q", result.Spaces[0].Name)
		}

		gotColls := map[string]model.DocsHelpcenterCollectionTranslation{}
		for _, c := range result.Collections {
			gotColls[c.CollectionID] = c
		}
		if gotColls[collDescID].Name != "Commencer" {
			t.Errorf("coll-desc name = %q, want Commencer", gotColls[collDescID].Name)
		}
		if gotColls[collDescID].Description == nil || *gotColls[collDescID].Description != "Bienvenue dans notre produit." {
			t.Errorf("coll-desc description not saved")
		}
		if gotColls[collNoDescID].Name != "Facturation" {
			t.Errorf("coll-nodesc name = %q, want Facturation", gotColls[collNoDescID].Name)
		}
	})

	t.Run("zero targets via pre-seeded rows short-circuits LLM", func(t *testing.T) {
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		provider := &scriptedDocsTranslationLLM{response: `should not be called`}
		svc := newDocsHelpcenterTranslationServiceForTestWithLLM(db, provider)

		seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
			ID:                      "cfg-zero",
			WorkspaceID:             workspaceID,
			Subdomain:               "zero",
			BrandName:               "Zero",
			BrandColor:              "#000000",
			ThemeMode:               "light",
			DefaultLocale:           "en",
			EnabledLocales:          model.DocsStringArray{"en", "fr"},
			FallbackToDefaultLocale: true,
			CreatedAt:               now,
			UpdatedAt:               now,
		})
		// No spaces seeded — nothing to translate.

		result, err := svc.AutoTranslateMissing(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("AutoTranslateMissing: %v", err)
		}
		if result.Requested != 0 {
			t.Errorf("Requested = %d, want 0", result.Requested)
		}
		if len(provider.requests) != 0 {
			t.Errorf("LLM was called for zero-target case: %d calls", len(provider.requests))
		}
	})

	t.Run("missing llm output marks target as failed, others succeed", func(t *testing.T) {
		// LLM returns index 1 and 3, drops index 2. Expect space and
		// coll-nodesc to be created, coll-desc to be in Failed.
		llmResponse := `[
			{"index": 1, "name": "Centre d'aide", "description": ""},
			{"index": 3, "name": "Facturation", "description": ""}
		]`
		svc, _, _, collDescID, _ := setup(t, llmResponse, nil)

		result, err := svc.AutoTranslateMissing(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("AutoTranslateMissing: %v", err)
		}
		if len(result.Spaces) != 1 {
			t.Errorf("Spaces len = %d, want 1", len(result.Spaces))
		}
		if len(result.Collections) != 1 {
			t.Errorf("Collections len = %d, want 1", len(result.Collections))
		}
		if len(result.Failed) != 1 {
			t.Fatalf("Failed len = %d, want 1", len(result.Failed))
		}
		if result.Failed[0].Kind != "collection" || result.Failed[0].ID != collDescID {
			t.Errorf("Failed item = %+v, want coll-desc", result.Failed[0])
		}
	})

	t.Run("out-of-range indices from llm are dropped, targets marked failed", func(t *testing.T) {
		// LLM returns an index that doesn't exist (99). No valid
		// items at all — expect all targets to be marked failed and
		// no rows created.
		llmResponse := `[{"index": 99, "name": "Ghost", "description": ""}]`
		svc, _, _, _, _ := setup(t, llmResponse, nil)

		result, err := svc.AutoTranslateMissing(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("AutoTranslateMissing: %v", err)
		}
		if len(result.Spaces) != 0 || len(result.Collections) != 0 {
			t.Errorf("nothing should have been written; got %d spaces, %d colls",
				len(result.Spaces), len(result.Collections))
		}
		if len(result.Failed) != 3 {
			t.Errorf("Failed len = %d, want 3 (all targets)", len(result.Failed))
		}
	})

	t.Run("llm error propagates as service error", func(t *testing.T) {
		llmErr := errors.New("model is busy")
		svc, _, _, _, _ := setup(t, "", llmErr)

		_, err := svc.AutoTranslateMissing(ctx, workspaceID, "fr")
		if err == nil {
			t.Fatalf("expected error from llm failure")
		}
	})

	t.Run("existing row is not overwritten (race guard)", func(t *testing.T) {
		// Pre-seed a translation row for the space. After the LLM
		// call, the re-check must skip writing that row — the
		// existing name stays.
		db := setupDocsHelpcenterTranslationServiceTestDB(t)
		provider := &scriptedDocsTranslationLLM{response: `[
			{"index": 1, "name": "Should Not Overwrite", "description": ""},
			{"index": 2, "name": "Commencer", "description": ""},
			{"index": 3, "name": "Facturation", "description": ""}
		]`}
		svc := newDocsHelpcenterTranslationServiceForTestWithLLM(db, provider)

		seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
			ID:                      "cfg-race",
			WorkspaceID:             workspaceID,
			Subdomain:               "race",
			BrandName:               "Race",
			BrandColor:              "#000000",
			ThemeMode:               "light",
			DefaultLocale:           "en",
			EnabledLocales:          model.DocsStringArray{"en", "fr"},
			FallbackToDefaultLocale: true,
			CreatedAt:               now,
			UpdatedAt:               now,
		})
		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          "space-auto",
			WorkspaceID: workspaceID,
			Name:        "Help Center",
			Slug:        "help-center",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-desc",
			SpaceID:     "space-auto",
			WorkspaceID: workspaceID,
			Name:        "Getting Started",
			Slug:        "getting-started",
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          "coll-nodesc",
			SpaceID:     "space-auto",
			WorkspaceID: workspaceID,
			Name:        "Billing",
			Slug:        "billing",
			Position:    1,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		// This collect cycle will run before we pre-seed the space
		// translation so the space IS collected as a target. We
		// seed the translation between collect and write by using
		// a fresh run where the translation already exists.
		seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
			ID:          "pre-existing",
			SpaceID:     "space-auto",
			WorkspaceID: workspaceID,
			Locale:      "fr",
			Name:        "Pre-existing Name",
			Slug:        pstr("pre-existing"),
			Status:      model.DocsHelpcenterTranslationStatusDraft,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		result, err := svc.AutoTranslateMissing(ctx, workspaceID, "fr")
		if err != nil {
			t.Fatalf("AutoTranslateMissing: %v", err)
		}
		// Since the space is now pre-filled, only the two collections
		// are targets. The service should write both and not touch
		// the pre-existing space row.
		if result.Requested != 2 {
			t.Errorf("Requested = %d, want 2 (space was pre-filled)", result.Requested)
		}
		if len(result.Collections) != 2 {
			t.Errorf("Collections len = %d, want 2", len(result.Collections))
		}
		if len(result.Spaces) != 0 {
			t.Errorf("Spaces len = %d, want 0 (pre-filled)", len(result.Spaces))
		}
	})
}

