package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsCollectionRepository handles DB operations for docs collections.
type DocsCollectionRepository struct {
	db *gorm.DB
}

// NewDocsCollectionRepository creates a new DocsCollectionRepository.
func NewDocsCollectionRepository(db *gorm.DB) *DocsCollectionRepository {
	return &DocsCollectionRepository{db: db}
}

// Create inserts a new collection. If the incoming struct has no ID set,
// a v4 UUID is generated here so SQLite-backed unit tests do not rely on
// Postgres's pgcrypto default. In production the result is identical to
// using the DB default — a v4 UUID string — because GORM sends the ID as
// part of the INSERT whether it was generated in Go or by the DB.
func (r *DocsCollectionRepository) Create(ctx context.Context, coll *model.DocsCollection) (*model.DocsCollection, error) {
	if coll.ID == "" {
		coll.ID = uuid.NewString()
	}
	if err := r.db.WithContext(ctx).Create(coll).Error; err != nil {
		return nil, fmt.Errorf("create docs collection: %w", err)
	}
	return coll, nil
}

// GetByID returns a collection by ID.
func (r *DocsCollectionRepository) GetByID(ctx context.Context, id string) (*model.DocsCollection, error) {
	var coll model.DocsCollection
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&coll).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs collection: %w", err)
	}
	return &coll, nil
}

// ListBySpace returns all collections in a space, ordered by position.
func (r *DocsCollectionRepository) ListBySpace(ctx context.Context, spaceID string) ([]model.DocsCollection, error) {
	var colls []model.DocsCollection
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Order("position ASC, created_at ASC").
		Find(&colls).Error; err != nil {
		return nil, fmt.Errorf("list docs collections: %w", err)
	}
	return colls, nil
}

// ListByWorkspace returns all collections across all spaces in a workspace.
func (r *DocsCollectionRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.DocsCollection, error) {
	var colls []model.DocsCollection
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID).
		Order("space_id ASC, position ASC, created_at ASC").
		Find(&colls).Error; err != nil {
		return nil, fmt.Errorf("list docs collections by workspace: %w", err)
	}
	return colls, nil
}

// Update applies partial updates to a collection.
func (r *DocsCollectionRepository) Update(ctx context.Context, id string, updates map[string]interface{}) (*model.DocsCollection, error) {
	if err := r.db.WithContext(ctx).Model(&model.DocsCollection{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update docs collection: %w", err)
	}
	return r.GetByID(ctx, id)
}

// Delete soft-deletes a collection while preserving every descendant
// collection and every direct article. Tree-aware delete rules (Task 5):
//
//   - Direct child collections are reparented to the deleted node's parent
//     (or become top-level when the deleted node was top-level). Their
//     depth is recalculated and propagated through the subtree.
//   - Direct articles move to the deleted node's parent collection (or
//     become uncategorized when the deleted node was top-level).
//   - The deleted node's sibling bucket and all destination buckets are
//     normalized at the end so positions stay contiguous.
//
// This preserves hierarchy continuity: deleting a middle collection
// flattens its content up one level rather than orphaning it or pushing
// everything into a global uncategorized bucket.
func (r *DocsCollectionRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var coll model.DocsCollection
		if err := tx.Where("id = ? AND deleted_at IS NULL", id).First(&coll).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("get docs collection for delete: %w", err)
		}

		destParentID := coll.ParentCollectionID
		destDepth, err := resolveDepthForParentTx(tx, destParentID)
		if err != nil {
			return err
		}

		// Reparent direct child collections to the destination bucket.
		var children []model.DocsCollection
		if err := tx.
			Where("parent_collection_id = ? AND deleted_at IS NULL", id).
			Order("position ASC, created_at ASC, id ASC").
			Find(&children).Error; err != nil {
			return fmt.Errorf("load child collections for delete: %w", err)
		}

		// Count current occupants of the destination collection bucket
		// INCLUDING the node we are about to delete. The deleted node is
		// itself a member of its parent's bucket until the soft-delete
		// lands, so counting it ensures appended children land strictly
		// after every existing row. The final normalize step compacts
		// the gap left by the soft-delete.
		var destCollCount int64
		destCollQuery := tx.Model(&model.DocsCollection{}).
			Where("space_id = ? AND deleted_at IS NULL", coll.SpaceID)
		if destParentID == nil {
			destCollQuery = destCollQuery.Where("parent_collection_id IS NULL")
		} else {
			destCollQuery = destCollQuery.Where("parent_collection_id = ?", *destParentID)
		}
		if err := destCollQuery.Count(&destCollCount).Error; err != nil {
			return fmt.Errorf("count destination collection bucket: %w", err)
		}

		for i, child := range children {
			updates := map[string]interface{}{
				"parent_collection_id": destParentID,
				"depth":                destDepth,
				"position":             int(destCollCount) + i,
			}
			if err := tx.Model(&model.DocsCollection{}).
				Where("id = ? AND deleted_at IS NULL", child.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("reparent child collection %s: %w", child.ID, err)
			}
			if err := recalculateDescendantDepthsTx(tx, child.ID, destDepth); err != nil {
				return err
			}
		}

		// Move direct articles to the destination article bucket.
		var docs []model.DocsDocument
		if err := tx.
			Where("collection_id = ? AND deleted_at IS NULL", id).
			Order("position ASC, created_at ASC, id ASC").
			Find(&docs).Error; err != nil {
			return fmt.Errorf("list docs in collection for delete: %w", err)
		}

		var destDocCount int64
		destDocQuery := tx.Model(&model.DocsDocument{}).
			Where("space_id = ? AND deleted_at IS NULL", coll.SpaceID)
		if destParentID == nil {
			destDocQuery = destDocQuery.Where("collection_id IS NULL")
		} else {
			destDocQuery = destDocQuery.Where("collection_id = ?", *destParentID)
		}
		if err := destDocQuery.Count(&destDocCount).Error; err != nil {
			return fmt.Errorf("count destination document bucket: %w", err)
		}

		for i, doc := range docs {
			updates := map[string]interface{}{
				"collection_id": destParentID,
				"position":      int(destDocCount) + i,
			}
			if err := tx.Model(&model.DocsDocument{}).
				Where("id = ? AND deleted_at IS NULL", doc.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("move doc %s to destination bucket: %w", doc.ID, err)
			}
		}

		// Soft-delete the collection itself.
		if err := tx.Model(&model.DocsCollection{}).
			Where("id = ? AND deleted_at IS NULL", id).
			Update("deleted_at", time.Now().UTC()).Error; err != nil {
			return fmt.Errorf("delete docs collection: %w", err)
		}

		// Normalize affected buckets. The collection's old sibling bucket
		// is the same as the destination bucket because the children were
		// flattened up one level. A single normalize call is enough for
		// the collection side. The document side mirrors this — the
		// deleted node's doc bucket no longer exists, and its articles
		// now live in destParentID's article bucket.
		if err := normalizeBucketTx(tx, coll.SpaceID, destParentID); err != nil {
			return err
		}
		return normalizeDocumentBucketTx(tx, coll.SpaceID, destParentID)
	})
}

// Restore un-deletes a collection.
func (r *DocsCollectionRepository) Restore(ctx context.Context, id string) (*model.DocsCollection, error) {
	if err := r.db.WithContext(ctx).Exec("UPDATE docs_collections SET deleted_at = NULL WHERE id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("restore docs collection: %w", err)
	}
	return r.GetByID(ctx, id)
}

// Reorder sets contiguous positions for the given collection IDs within a space.
func (r *DocsCollectionRepository) Reorder(ctx context.Context, spaceID string, orderedIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range orderedIDs {
			if err := tx.Model(&model.DocsCollection{}).
				Where("id = ? AND space_id = ? AND deleted_at IS NULL", id, spaceID).
				UpdateColumn("position", i).Error; err != nil {
				return fmt.Errorf("reorder collection %s: %w", id, err)
			}
		}
		return nil
	})
}

// NormalizeSpace re-numbers collection positions in every sibling bucket in
// the space. Each (parent_collection_id) bucket becomes contiguous starting
// from 0, preserving relative order. Safe to call after reorders, reparents,
// and deletes.
func (r *DocsCollectionRepository) NormalizeSpace(ctx context.Context, spaceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return normalizeAllBucketsInSpaceTx(tx, spaceID)
	})
}

// NormalizeBucket re-numbers positions within a single (space_id,
// parent_collection_id) sibling bucket. parentID == nil targets the
// top-level bucket in the space.
func (r *DocsCollectionRepository) NormalizeBucket(ctx context.Context, spaceID string, parentID *string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return normalizeBucketTx(tx, spaceID, parentID)
	})
}

// ListChildren returns the direct children of parentID in a space (or all
// top-level collections when parentID is nil), ordered by position.
func (r *DocsCollectionRepository) ListChildren(ctx context.Context, spaceID string, parentID *string) ([]model.DocsCollection, error) {
	var colls []model.DocsCollection
	q := r.db.WithContext(ctx).
		Where("space_id = ? AND deleted_at IS NULL", spaceID)
	if parentID == nil {
		q = q.Where("parent_collection_id IS NULL")
	} else {
		q = q.Where("parent_collection_id = ?", *parentID)
	}
	if err := q.Order("position ASC, created_at ASC, id ASC").Find(&colls).Error; err != nil {
		return nil, fmt.Errorf("list docs collection children: %w", err)
	}
	return colls, nil
}

// ListAncestors walks from collectionID up to the top of the tree and
// returns the ancestor chain ordered from the immediate parent to the
// root. It is iterative and capped at maxCollectionTreeDepth levels, which
// matches the depth cap enforced at the service layer. Missing or dangling
// parent references terminate the walk cleanly.
func (r *DocsCollectionRepository) ListAncestors(ctx context.Context, collectionID string) ([]model.DocsCollection, error) {
	var ancestors []model.DocsCollection
	current, err := r.GetByID(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, nil
	}
	for i := 0; i < maxCollectionTreeDepth; i++ {
		if current.ParentCollectionID == nil {
			return ancestors, nil
		}
		parent, err := r.GetByID(ctx, *current.ParentCollectionID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			return ancestors, nil
		}
		ancestors = append(ancestors, *parent)
		current = parent
	}
	return ancestors, nil
}

// ListDescendants returns every descendant of collectionID via breadth-first
// iteration capped at maxCollectionTreeDepth levels. The root itself is not
// included in the result. Order is stable: each level is listed before the
// next, and within a level rows are ordered by position then created_at.
func (r *DocsCollectionRepository) ListDescendants(ctx context.Context, collectionID string) ([]model.DocsCollection, error) {
	root, err := r.GetByID(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return nil, nil
	}

	var result []model.DocsCollection
	frontier := []string{collectionID}
	for level := 0; level < maxCollectionTreeDepth && len(frontier) > 0; level++ {
		var nextFrontier []string
		for _, parentID := range frontier {
			pid := parentID
			children, err := r.ListChildren(ctx, root.SpaceID, &pid)
			if err != nil {
				return nil, err
			}
			result = append(result, children...)
			for _, c := range children {
				nextFrontier = append(nextFrontier, c.ID)
			}
		}
		frontier = nextFrontier
	}
	return result, nil
}

// NextPositionInBucket returns the next free position in a specific sibling
// bucket. parentID == nil targets the top-level bucket in the space.
// Normalizes the bucket before reading max+1 to avoid drift after deletes.
func (r *DocsCollectionRepository) NextPositionInBucket(ctx context.Context, spaceID string, parentID *string) (int, error) {
	if err := r.NormalizeBucket(ctx, spaceID, parentID); err != nil {
		return 0, err
	}
	var maxPos *int
	q := r.db.WithContext(ctx).
		Model(&model.DocsCollection{}).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Select("COALESCE(MAX(position), -1)")
	if parentID == nil {
		q = q.Where("parent_collection_id IS NULL")
	} else {
		q = q.Where("parent_collection_id = ?", *parentID)
	}
	if err := q.Scan(&maxPos).Error; err != nil {
		return 0, fmt.Errorf("next collection position in bucket: %w", err)
	}
	if maxPos == nil {
		return 0, nil
	}
	return *maxPos + 1, nil
}

// ReorderSiblings sets contiguous positions for the given collection IDs
// within one (space_id, parent_collection_id) sibling bucket. IDs that do
// not belong to that bucket are ignored so a caller cannot accidentally
// displace sibling groups with a stale or mis-scoped payload.
func (r *DocsCollectionRepository) ReorderSiblings(ctx context.Context, spaceID string, parentID *string, orderedIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range orderedIDs {
			q := tx.Model(&model.DocsCollection{}).
				Where("id = ? AND space_id = ? AND deleted_at IS NULL", id, spaceID)
			if parentID == nil {
				q = q.Where("parent_collection_id IS NULL")
			} else {
				q = q.Where("parent_collection_id = ?", *parentID)
			}
			if err := q.UpdateColumn("position", i).Error; err != nil {
				return fmt.Errorf("reorder sibling %s: %w", id, err)
			}
		}
		return normalizeBucketTx(tx, spaceID, parentID)
	})
}

// Reparent moves a collection to a new parent within the same space,
// recalculates the depth of the moved node and every descendant, and
// normalizes both the old and new sibling buckets. newParentID == nil
// reparents to the top of the space. This method is a dumb mover — it
// does not enforce the depth cap or prevent cycles. Both invariants live
// in the service layer (Task 3) so the repo remains reusable for tests
// and future data migrations.
func (r *DocsCollectionRepository) Reparent(ctx context.Context, collectionID string, newParentID *string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var coll model.DocsCollection
		if err := tx.Where("id = ? AND deleted_at IS NULL", collectionID).First(&coll).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("get collection for reparent: %w", err)
		}

		newDepth, err := resolveDepthForParentTx(tx, newParentID)
		if err != nil {
			return err
		}

		oldParentID := coll.ParentCollectionID

		// Append to the end of the new sibling bucket. Counting current
		// rows in the bucket (excluding the moved row itself) gives us an
		// unambiguous trailing position independent of the moved row's
		// stale Position value from its old bucket.
		var newPos int64
		bucketCount := tx.Model(&model.DocsCollection{}).
			Where("space_id = ? AND deleted_at IS NULL AND id <> ?", coll.SpaceID, collectionID)
		if newParentID == nil {
			bucketCount = bucketCount.Where("parent_collection_id IS NULL")
		} else {
			bucketCount = bucketCount.Where("parent_collection_id = ?", *newParentID)
		}
		if err := bucketCount.Count(&newPos).Error; err != nil {
			return fmt.Errorf("count new bucket for reparent: %w", err)
		}

		updates := map[string]interface{}{
			"parent_collection_id": newParentID,
			"depth":                newDepth,
			"position":             int(newPos),
		}
		if err := tx.Model(&model.DocsCollection{}).
			Where("id = ? AND deleted_at IS NULL", collectionID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("reparent collection %s: %w", collectionID, err)
		}

		if err := recalculateDescendantDepthsTx(tx, collectionID, newDepth); err != nil {
			return err
		}

		if err := normalizeBucketTx(tx, coll.SpaceID, oldParentID); err != nil {
			return err
		}
		if err := normalizeBucketTx(tx, coll.SpaceID, newParentID); err != nil {
			return err
		}
		return nil
	})
}

// maxCollectionTreeDepth is the number of distinct levels in the collection
// tree, inclusive of the root. The depth column stores 0..maxCollectionTreeDepth-1.
// The service layer enforces this cap — the repository respects it when
// iterating ancestors and descendants so a dangling or corrupted parent
// reference cannot cause an infinite loop.
const maxCollectionTreeDepth = 3

func resolveDepthForParentTx(tx *gorm.DB, parentID *string) (int, error) {
	if parentID == nil {
		return 0, nil
	}
	var parent model.DocsCollection
	if err := tx.Where("id = ? AND deleted_at IS NULL", *parentID).First(&parent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("reparent: parent collection %s not found", *parentID)
		}
		return 0, fmt.Errorf("load parent collection for reparent: %w", err)
	}
	return parent.Depth + 1, nil
}

func recalculateDescendantDepthsTx(tx *gorm.DB, rootID string, rootDepth int) error {
	frontier := []struct {
		id    string
		depth int
	}{{id: rootID, depth: rootDepth}}

	for level := 0; level < maxCollectionTreeDepth && len(frontier) > 0; level++ {
		var next []struct {
			id    string
			depth int
		}
		for _, node := range frontier {
			var children []model.DocsCollection
			if err := tx.
				Where("parent_collection_id = ? AND deleted_at IS NULL", node.id).
				Find(&children).Error; err != nil {
				return fmt.Errorf("load children for depth recalc: %w", err)
			}
			childDepth := node.depth + 1
			for _, child := range children {
				if child.Depth != childDepth {
					if err := tx.Model(&model.DocsCollection{}).
						Where("id = ? AND deleted_at IS NULL", child.ID).
						UpdateColumn("depth", childDepth).Error; err != nil {
						return fmt.Errorf("update descendant depth %s: %w", child.ID, err)
					}
				}
				next = append(next, struct {
					id    string
					depth int
				}{id: child.ID, depth: childDepth})
			}
		}
		frontier = next
	}
	return nil
}

func normalizeBucketTx(tx *gorm.DB, spaceID string, parentID *string) error {
	var colls []model.DocsCollection
	q := tx.Where("space_id = ? AND deleted_at IS NULL", spaceID)
	if parentID == nil {
		q = q.Where("parent_collection_id IS NULL")
	} else {
		q = q.Where("parent_collection_id = ?", *parentID)
	}
	if err := q.
		Order("position ASC, created_at ASC, id ASC").
		Find(&colls).Error; err != nil {
		return fmt.Errorf("list collections for bucket normalization: %w", err)
	}

	for i, coll := range colls {
		if coll.Position == i {
			continue
		}
		if err := tx.Model(&model.DocsCollection{}).
			Where("id = ? AND deleted_at IS NULL", coll.ID).
			UpdateColumn("position", i).Error; err != nil {
			return fmt.Errorf("normalize collection %s: %w", coll.ID, err)
		}
	}

	return nil
}

func normalizeAllBucketsInSpaceTx(tx *gorm.DB, spaceID string) error {
	// Collect every distinct parent_collection_id value present in the
	// space (including NULL for the top-level bucket), then normalize each
	// bucket independently. A bounded set of distinct parents keeps this
	// cheap even for moderate tree sizes.
	type bucketRow struct {
		ParentCollectionID *string
	}
	var buckets []bucketRow
	if err := tx.Model(&model.DocsCollection{}).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Distinct("parent_collection_id").
		Scan(&buckets).Error; err != nil {
		return fmt.Errorf("list buckets for space normalization: %w", err)
	}
	for _, b := range buckets {
		if err := normalizeBucketTx(tx, spaceID, b.ParentCollectionID); err != nil {
			return err
		}
	}
	return nil
}

