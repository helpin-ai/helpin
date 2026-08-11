package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsRedirectRepository handles DB operations for docs URL redirects.
type DocsRedirectRepository struct {
	db *gorm.DB
}

// NewDocsRedirectRepository creates a new DocsRedirectRepository.
func NewDocsRedirectRepository(db *gorm.DB) *DocsRedirectRepository {
	return &DocsRedirectRepository{db: db}
}

func normalizeDocsRedirectSourcePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '/'
	})
	if len(parts) == 0 {
		return "/"
	}

	return "/" + strings.Join(parts, "/")
}

func normalizeDocsRedirectSlugPart(value string) string {
	return strings.Trim(strings.TrimSpace(value), "/")
}

var docsRedirectSlugSanitizer = regexp.MustCompile(`[^a-z0-9-]+`)

func slugifyDocsRedirectPart(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = docsRedirectSlugSanitizer.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return slug
}

func normalizeDocsRedirectRecord(redirect *model.DocsRedirect) {
	redirect.SourcePath = normalizeDocsRedirectSourcePath(redirect.SourcePath)
	redirect.TargetCollectionSlug = normalizeDocsRedirectSlugPart(redirect.TargetCollectionSlug)
	if redirect.TargetArticleSlug == nil {
		return
	}

	normalizedArticle := normalizeDocsRedirectSlugPart(*redirect.TargetArticleSlug)
	if normalizedArticle == "" {
		redirect.TargetArticleSlug = nil
		return
	}
	redirect.TargetArticleSlug = &normalizedArticle
}

func sameRedirectTargetArticle(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func redirectSourceArticleSlug(sourcePath string) string {
	segments := strings.Split(strings.Trim(sourcePath, "/"), "/")
	if len(segments) == 0 {
		return ""
	}
	return strings.TrimSpace(segments[len(segments)-1])
}

func (r *DocsRedirectRepository) resolveCollectionSlugForArticle(ctx context.Context, articleSlug string) (string, error) {
	if articleSlug == "" {
		return "", nil
	}

	type articleCollectionRow struct {
		CollectionID   *string
		CollectionSlug *string
		CollectionName *string
	}

	var row articleCollectionRow
	err := r.db.WithContext(ctx).
		Table("docs_helpcenter_articles AS h").
		Select("d.collection_id AS collection_id, c.slug AS collection_slug, c.name AS collection_name").
		Joins("JOIN docs_documents d ON d.id = h.document_id").
		Joins("LEFT JOIN docs_collections c ON c.id = d.collection_id").
		Where("h.slug = ?", articleSlug).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return "", fmt.Errorf("resolve collection slug for article %q: %w", articleSlug, err)
	}
	if row.CollectionID == nil {
		return "", nil
	}

	collectionSlug := normalizeDocsRedirectSlugPart(valueOrEmpty(row.CollectionSlug))
	if collectionSlug != "" {
		return collectionSlug, nil
	}

	collectionSlug = slugifyDocsRedirectPart(valueOrEmpty(row.CollectionName))
	if collectionSlug == "" {
		return "", nil
	}

	if err := r.db.WithContext(ctx).
		Table("docs_collections").
		Where("id = ?", *row.CollectionID).
		Update("slug", collectionSlug).Error; err != nil {
		return "", fmt.Errorf("backfill docs collection slug %s: %w", *row.CollectionID, err)
	}

	return collectionSlug, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeDocsRedirectUpdates(updates map[string]interface{}) {
	if raw, ok := updates["source_path"].(string); ok {
		updates["source_path"] = normalizeDocsRedirectSourcePath(raw)
	}
	if raw, ok := updates["target_collection_slug"].(string); ok {
		updates["target_collection_slug"] = normalizeDocsRedirectSlugPart(raw)
	}
	if raw, ok := updates["target_article_slug"].(string); ok {
		normalized := normalizeDocsRedirectSlugPart(raw)
		if normalized == "" {
			updates["target_article_slug"] = nil
		} else {
			updates["target_article_slug"] = normalized
		}
	}
}

func (r *DocsRedirectRepository) repairMalformedPaths(ctx context.Context, workspaceID string) error {
	var redirects []model.DocsRedirect
	if err := r.db.WithContext(ctx).
		Where(
			`workspace_id = ? AND (
				source_path LIKE ? OR source_path LIKE ? OR
				target_collection_slug LIKE ? OR target_collection_slug LIKE ? OR
				target_article_slug LIKE ? OR target_article_slug LIKE ? OR
				(type = ? AND COALESCE(target_collection_slug, '') = '' AND target_article_slug IS NOT NULL)
			)`,
			workspaceID,
			"//%", "%//%",
			"/%", "%/",
			"/%", "%/",
			model.RedirectTypeSlugChange,
		).
		Find(&redirects).Error; err != nil {
		return fmt.Errorf("load malformed docs redirects: %w", err)
	}

	for _, redirect := range redirects {
		normalized := redirect
		normalizeDocsRedirectRecord(&normalized)

		if normalized.TargetCollectionSlug == "" && normalized.TargetArticleSlug != nil {
			collectionSlug, err := r.resolveCollectionSlugForArticle(ctx, *normalized.TargetArticleSlug)
			if err != nil {
				return err
			}
			if collectionSlug != "" {
				normalized.TargetCollectionSlug = collectionSlug
				if sourceArticleSlug := redirectSourceArticleSlug(normalized.SourcePath); sourceArticleSlug != "" {
					normalized.SourcePath = normalizeDocsRedirectSourcePath(collectionSlug + "/" + sourceArticleSlug)
				}
			}
		}

		if normalized.SourcePath == redirect.SourcePath &&
			normalized.TargetCollectionSlug == redirect.TargetCollectionSlug &&
			sameRedirectTargetArticle(normalized.TargetArticleSlug, redirect.TargetArticleSlug) {
			continue
		}

		updates := map[string]interface{}{
			"source_path":            normalized.SourcePath,
			"target_collection_slug": normalized.TargetCollectionSlug,
			"target_article_slug":    normalized.TargetArticleSlug,
		}
		if err := r.db.WithContext(ctx).
			Model(&model.DocsRedirect{}).
			Where("id = ?", redirect.ID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("repair docs redirect %s: %w", redirect.ID, err)
		}
	}

	return nil
}

// Create inserts a single redirect record.
func (r *DocsRedirectRepository) Create(ctx context.Context, redirect *model.DocsRedirect) error {
	normalizeDocsRedirectRecord(redirect)
	if err := r.db.WithContext(ctx).Create(redirect).Error; err != nil {
		return fmt.Errorf("create docs redirect: %w", err)
	}
	return nil
}

// UpsertImported inserts an imported redirect or refreshes its destination
// when the same source object is imported again. Redirects owned by a user or
// by a different import source are preserved.
func (r *DocsRedirectRepository) UpsertImported(ctx context.Context, redirect *model.DocsRedirect) (bool, error) {
	normalizeDocsRedirectRecord(redirect)
	if redirect.Type != model.RedirectTypeImported {
		return false, fmt.Errorf("UpsertImported requires an imported redirect, got %q", redirect.Type)
	}

	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "source_path"}},
			DoNothing: true,
		}).Create(redirect)
		if result.Error != nil {
			return fmt.Errorf("insert imported docs redirect: %w", result.Error)
		}
		if result.RowsAffected == 1 {
			created = true
			return nil
		}

		var existing model.DocsRedirect
		if err := tx.
			Where("workspace_id = ? AND source_path = ?", redirect.WorkspaceID, redirect.SourcePath).
			First(&existing).Error; err != nil {
			return fmt.Errorf("load existing imported docs redirect: %w", err)
		}
		if existing.Type != model.RedirectTypeImported || !sameImportedRedirectSource(existing, *redirect) {
			return nil
		}

		updates := map[string]interface{}{
			"target_collection_slug": redirect.TargetCollectionSlug,
			"target_article_slug":    redirect.TargetArticleSlug,
			"target_path":            redirect.TargetPath,
			"source_system":          redirect.SourceSystem,
			"source_object_type":     redirect.SourceObjectType,
			"source_object_id":       redirect.SourceObjectID,
		}
		if err := tx.Model(&model.DocsRedirect{}).
			Where("id = ?", existing.ID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("refresh imported docs redirect: %w", err)
		}
		return nil
	})
	return created, err
}

func sameImportedRedirectSource(existing, incoming model.DocsRedirect) bool {
	if existing.SourceSystem != nil && incoming.SourceSystem != nil &&
		!strings.EqualFold(strings.TrimSpace(*existing.SourceSystem), strings.TrimSpace(*incoming.SourceSystem)) {
		return false
	}
	if existing.SourceObjectID != nil && incoming.SourceObjectID != nil &&
		strings.TrimSpace(*existing.SourceObjectID) != strings.TrimSpace(*incoming.SourceObjectID) {
		return false
	}
	return true
}

// isAutoRedirectType reports whether a redirect type is one of the
// automatic kinds emitted by the tree rollout. Manual and imported
// redirects are preserved across automatic moves/renames so that
// user-configured redirects are never silently overwritten.
func isAutoRedirectType(t string) bool {
	switch t {
	case model.RedirectTypeAutoArticleMove,
		model.RedirectTypeAutoCollectionRename,
		model.RedirectTypeSlugChange:
		return true
	}
	return false
}

// UpsertWithReconciliation inserts or replaces an automatic redirect
// while breaking any cycle it would create.
//
// The input redirect must be an automatic type (auto_article_move,
// auto_collection_rename, or the legacy slug_change). This method will
// refuse to mutate a manual or imported redirect under any branch:
//
//   - Before inserting, any auto-generated redirect whose source path
//     equals the new target path is deleted so an A -> B -> A move
//     collapses back to a single hop. Manual/imported redirects at
//     the same path are left in place.
//   - Before inserting, any existing row at the new SOURCE path is
//     inspected. If the existing row is manual or imported, the
//     entire upsert is skipped — the user-configured redirect wins.
//     If the existing row is auto, its target + type are replaced so
//     a repeated move from the same origin is idempotent.
//
// If the computed source and target paths are identical the call is a
// no-op (a redirect to itself would loop forever).
func (r *DocsRedirectRepository) UpsertWithReconciliation(ctx context.Context, redirect *model.DocsRedirect) error {
	normalizeDocsRedirectRecord(redirect)
	// Prefer pre-computed target_path; fall back to slug-based rebuild.
	var targetPath string
	if redirect.TargetPath != nil && *redirect.TargetPath != "" {
		targetPath = *redirect.TargetPath
	} else {
		targetPath = buildRedirectTargetPath(redirect.TargetCollectionSlug, redirect.TargetArticleSlug)
	}
	if redirect.SourcePath == "" || targetPath == "" {
		return fmt.Errorf("redirect source and target paths are required")
	}
	if redirect.SourcePath == targetPath {
		// No-op: pointing a path at itself would loop forever.
		return nil
	}
	if !isAutoRedirectType(redirect.Type) {
		return fmt.Errorf("UpsertWithReconciliation requires an automatic redirect type, got %q", redirect.Type)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Break any existing auto redirect chain where the new target
		// is currently a redirect source. Manual or imported redirects
		// at the same path are left in place — they represent explicit
		// user intent and must not be silently deleted by the
		// collection move/rename flow.
		if err := tx.
			Where(
				"workspace_id = ? AND source_path = ? AND type IN (?, ?, ?)",
				redirect.WorkspaceID, targetPath,
				model.RedirectTypeAutoArticleMove,
				model.RedirectTypeAutoCollectionRename,
				model.RedirectTypeSlugChange,
			).
			Delete(&model.DocsRedirect{}).Error; err != nil {
			return fmt.Errorf("delete stale auto redirect at target: %w", err)
		}

		// If a redirect already exists at the new source path, check
		// its type before deciding what to do. Manual and imported
		// redirects are preserved — user intent wins over automatic
		// rewrites. Auto redirects are replaced in place.
		var existing model.DocsRedirect
		err := tx.
			Where("workspace_id = ? AND source_path = ?", redirect.WorkspaceID, redirect.SourcePath).
			First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("lookup existing redirect at source: %w", err)
		}
		if err == nil && !isAutoRedirectType(existing.Type) {
			// User-configured row already lives here. Leave it alone.
			return nil
		}

		// Upsert on (workspace_id, source_path). If an auto row already
		// exists at this source path, replace its target + type so a
		// second move from the same origin overwrites the first.
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "workspace_id"}, {Name: "source_path"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"target_collection_slug",
				"target_article_slug",
				"target_path",
				"type",
			}),
		}).Create(redirect).Error
	})
}

// buildRedirectTargetPath mirrors buildDocsRedirectPath from the service
// layer but lives in the repo to avoid a dependency cycle. Both produce
// canonical public help-center paths from (collectionSlug, articleSlug).
func buildRedirectTargetPath(collectionSlug string, articleSlug *string) string {
	collection := collectionSlug
	article := ""
	if articleSlug != nil {
		article = *articleSlug
	}
	switch {
	case collection != "" && article != "":
		return "/" + collection + "/" + article
	case collection != "":
		return "/" + collection
	case article != "":
		return "/" + article
	default:
		return ""
	}
}

// BulkCreate inserts multiple redirect records, skipping duplicates on
// (workspace_id, source_path) conflicts.
func (r *DocsRedirectRepository) BulkCreate(ctx context.Context, redirects []model.DocsRedirect) error {
	if len(redirects) == 0 {
		return nil
	}
	for i := range redirects {
		normalizeDocsRedirectRecord(&redirects[i])
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&redirects).Error; err != nil {
		return fmt.Errorf("bulk create docs redirects: %w", err)
	}
	return nil
}

// GetBySourcePath returns the redirect matching the given workspace and source
// path. Returns nil (no error) when no record is found.
func (r *DocsRedirectRepository) GetBySourcePath(ctx context.Context, workspaceID, sourcePath string) (*model.DocsRedirect, error) {
	var redirect model.DocsRedirect
	normalizedPath := normalizeDocsRedirectSourcePath(sourcePath)
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND source_path = ?", workspaceID, normalizedPath).
		First(&redirect).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs redirect by source path: %w", err)
	}
	return &redirect, nil
}

// GetByID returns the redirect matching the given ID. Returns nil when absent.
func (r *DocsRedirectRepository) GetByID(ctx context.Context, id string) (*model.DocsRedirect, error) {
	var redirect model.DocsRedirect
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&redirect).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs redirect: %w", err)
	}
	return &redirect, nil
}

// List returns a paginated list of redirects for a workspace with optional
// search and type filters. Returns the items and total matching count.
func (r *DocsRedirectRepository) List(ctx context.Context, workspaceID string, filter model.DocsRedirectFilter) ([]model.DocsRedirect, int64, error) {
	if err := r.repairMalformedPaths(ctx, workspaceID); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	q := r.db.WithContext(ctx).Model(&model.DocsRedirect{}).Where("workspace_id = ?", workspaceID)

	if filter.Search != "" {
		q = q.Where("source_path ILIKE ?", "%"+filter.Search+"%")
	}
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count docs redirects: %w", err)
	}

	var items []model.DocsRedirect
	if err := q.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list docs redirects: %w", err)
	}

	return items, total, nil
}

// Update updates the target fields of a redirect by ID.
func (r *DocsRedirectRepository) Update(ctx context.Context, id string, updates map[string]interface{}) (*model.DocsRedirect, error) {
	normalizeDocsRedirectUpdates(updates)
	if err := r.db.WithContext(ctx).Model(&model.DocsRedirect{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update docs redirect: %w", err)
	}
	var redirect model.DocsRedirect
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&redirect).Error; err != nil {
		return nil, fmt.Errorf("get updated redirect: %w", err)
	}
	return &redirect, nil
}

// Delete removes a redirect by ID.
func (r *DocsRedirectRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.DocsRedirect{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete docs redirect: %w", err)
	}
	return nil
}
