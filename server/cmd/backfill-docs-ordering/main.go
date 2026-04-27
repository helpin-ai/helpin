// Command backfill-docs-ordering assigns fractional sort_key values to
// docs_collections and docs_documents that still have the default sentinel
// value ('~'). The generated keys preserve the current visual order:
// collections first (by position, created_at, id), then documents (by
// position, created_at, id) within each sibling bucket.
//
// Usage:
//
//	go run ./cmd/backfill-docs-ordering [--dry-run] [--workspace-id=UUID]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/ordering"
)

// defaultSortKey is the sentinel value that indicates a row has not been
// backfilled yet. Rows with this value are eligible for assignment.
const defaultSortKey = "~"

// collection is a minimal projection of docs_collections used during
// the backfill. We avoid importing model to keep this command lightweight.
type collection struct {
	ID                 string
	SpaceID            string
	ParentCollectionID *string
	Position           int
	SortKey            string
	CreatedAt          time.Time
}

// document is a minimal projection of docs_documents used during the backfill.
type document struct {
	ID           string
	SpaceID      string
	CollectionID *string
	Position     int
	SortKey      string
	CreatedAt    time.Time
}

// space is a minimal projection of docs_spaces.
type space struct {
	ID          string
	WorkspaceID string
}

func main() {
	dryRun := flag.Bool("dry-run", false, "compute keys without writing to the database")
	workspaceID := flag.String("workspace-id", "", "backfill a single workspace (empty = all)")
	flag.Parse()

	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	workspaceIDs, err := loadWorkspaceIDs(ctx, db, *workspaceID)
	if err != nil {
		log.Fatalf("load workspaces: %v", err)
	}
	log.Printf("workspaces to process: %d", len(workspaceIDs))

	var totalCollections, totalDocuments int

	for i, wsID := range workspaceIDs {
		nc, nd, err := processWorkspace(ctx, db, wsID, *dryRun)
		if err != nil {
			log.Fatalf("workspace %s: %v", wsID, err)
		}
		totalCollections += nc
		totalDocuments += nd
		if (i+1)%10 == 0 || i+1 == len(workspaceIDs) {
			log.Printf("progress: %d/%d workspaces", i+1, len(workspaceIDs))
		}
	}

	mode := "LIVE"
	if *dryRun {
		mode = "DRY-RUN"
	}
	log.Printf("[%s] done — %d collections, %d documents updated across %d workspaces",
		mode, totalCollections, totalDocuments, len(workspaceIDs))
}

// loadWorkspaceIDs returns the list of workspace IDs to process.
func loadWorkspaceIDs(ctx context.Context, db *gorm.DB, singleID string) ([]string, error) {
	if singleID != "" {
		return []string{singleID}, nil
	}
	var ids []string
	err := db.WithContext(ctx).
		Table("workspaces").
		Where("deleted_at IS NULL").
		Pluck("id", &ids).Error
	return ids, err
}

// processWorkspace backfills sort_keys for every space in a workspace,
// wrapped in a single transaction.
func processWorkspace(ctx context.Context, db *gorm.DB, workspaceID string, dryRun bool) (int, int, error) {
	var spaces []space
	if err := db.WithContext(ctx).
		Table("docs_spaces").
		Select("id, workspace_id").
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID).
		Find(&spaces).Error; err != nil {
		return 0, 0, fmt.Errorf("load spaces: %w", err)
	}

	if len(spaces) == 0 {
		return 0, 0, nil
	}

	var totalC, totalD int

	txFn := func(tx *gorm.DB) error {
		for _, sp := range spaces {
			nc, nd, err := walkBucket(ctx, tx, sp.ID, nil, dryRun)
			if err != nil {
				return fmt.Errorf("space %s: %w", sp.ID, err)
			}
			totalC += nc
			totalD += nd
		}
		return nil
	}

	if dryRun {
		// In dry-run mode, don't open a real transaction — just run the walk
		// read-only so we can log what would happen.
		if err := txFn(db.WithContext(ctx)); err != nil {
			return 0, 0, err
		}
	} else {
		if err := db.WithContext(ctx).Transaction(txFn); err != nil {
			return 0, 0, err
		}
	}

	if totalC+totalD > 0 {
		log.Printf("  workspace %s: %d spaces, %d collections, %d documents",
			workspaceID, len(spaces), totalC, totalD)
	}

	return totalC, totalD, nil
}

// walkBucket processes one sibling bucket (a space root or a collection's
// children). It assigns sort_keys to collections first, then documents,
// preserving the current visual order. It recurses into child collections.
func walkBucket(ctx context.Context, tx *gorm.DB, spaceID string, parentCollectionID *string, dryRun bool) (int, int, error) {
	// Load collections in this bucket.
	colQuery := tx.WithContext(ctx).
		Table("docs_collections").
		Select("id, space_id, parent_collection_id, position, sort_key, created_at").
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Order("position ASC, created_at ASC, id ASC")

	if parentCollectionID == nil {
		colQuery = colQuery.Where("parent_collection_id IS NULL")
	} else {
		colQuery = colQuery.Where("parent_collection_id = ?", *parentCollectionID)
	}

	var collections []collection
	if err := colQuery.Find(&collections).Error; err != nil {
		return 0, 0, fmt.Errorf("load collections: %w", err)
	}

	// Load documents in this bucket.
	docQuery := tx.WithContext(ctx).
		Table("docs_documents").
		Select("id, space_id, collection_id, position, sort_key, created_at").
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Order("position ASC, created_at ASC, id ASC")

	if parentCollectionID == nil {
		docQuery = docQuery.Where("collection_id IS NULL")
	} else {
		docQuery = docQuery.Where("collection_id = ?", *parentCollectionID)
	}

	var documents []document
	if err := docQuery.Find(&documents).Error; err != nil {
		return 0, 0, fmt.Errorf("load documents: %w", err)
	}

	// Assign sort_keys: collections first, then documents.
	prevKey := ""
	updatedC, updatedD := 0, 0

	for i := range collections {
		c := &collections[i]
		if c.SortKey != defaultSortKey {
			// Already backfilled — use its key as the previous bound.
			prevKey = c.SortKey
			continue
		}
		key, err := ordering.Between(prevKey, "")
		if err != nil {
			return 0, 0, fmt.Errorf("generate key for collection %s: %w", c.ID, err)
		}
		if !dryRun {
			if err := tx.WithContext(ctx).
				Table("docs_collections").
				Where("id = ?", c.ID).
				Update("sort_key", key).Error; err != nil {
				return 0, 0, fmt.Errorf("update collection %s: %w", c.ID, err)
			}
		}
		prevKey = key
		updatedC++
	}

	for i := range documents {
		d := &documents[i]
		if d.SortKey != defaultSortKey {
			prevKey = d.SortKey
			continue
		}
		key, err := ordering.Between(prevKey, "")
		if err != nil {
			return 0, 0, fmt.Errorf("generate key for document %s: %w", d.ID, err)
		}
		if !dryRun {
			if err := tx.WithContext(ctx).
				Table("docs_documents").
				Where("id = ?", d.ID).
				Update("sort_key", key).Error; err != nil {
				return 0, 0, fmt.Errorf("update document %s: %w", d.ID, err)
			}
		}
		prevKey = key
		updatedD++
	}

	// Recurse into child collections.
	for _, c := range collections {
		cID := c.ID
		nc, nd, err := walkBucket(ctx, tx, spaceID, &cID, dryRun)
		if err != nil {
			return 0, 0, err
		}
		updatedC += nc
		updatedD += nd
	}

	return updatedC, updatedD, nil
}
