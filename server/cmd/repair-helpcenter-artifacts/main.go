package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "repair failed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	apply := strings.EqualFold(strings.TrimSpace(os.Getenv("APPLY")), "true")
	documentIDs := parseDocumentIDs(os.Getenv("DOCUMENT_IDS"))

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DatabaseURL,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Error)})
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	artifactStore := storage.NewS3Client(
		cfg.AWSAccessKeyID,
		cfg.AWSSecretAccessKey,
		cfg.AWSBucket,
		cfg.AWSRegion,
		cfg.AWSEndpointURL,
		cfg.AWSPublicBaseURL,
		cfg.AWSPresignEndpointURL,
	)
	if artifactStore == nil {
		return fmt.Errorf("S3 storage is not configured")
	}

	publicationRepo := repository.NewDocsHelpcenterPublicationRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	publications, err := publicationRepo.ListAllArticlePublications(ctx)
	if err != nil {
		return err
	}

	affected := make([]model.DocsHelpcenterArticlePublication, 0)
	for _, publication := range publications {
		if len(documentIDs) > 0 {
			if _, ok := documentIDs[publication.DocumentID]; !ok {
				continue
			}
		}
		if service.HasActivePublicationArtifactReferences(publication.Content) {
			affected = append(affected, publication)
		}
	}

	mode := "dry-run"
	if apply {
		mode = "apply"
	}
	fmt.Printf("mode=%s scanned=%d affected=%d\n", mode, len(publications), len(affected))
	for _, publication := range affected {
		fmt.Printf("document_id=%s locale=%s workspace_id=%s\n", publication.DocumentID, publication.Locale, publication.WorkspaceID)
	}
	if !apply {
		return nil
	}

	repaired := 0
	for i := range affected {
		publication := &affected[i]
		materialized, err := service.MaterializeDocsPublicationArtifacts(
			ctx,
			artifactRepo,
			artifactStore,
			publication.WorkspaceID,
			publication.DocumentID,
			publication.Content,
		)
		if err != nil {
			return fmt.Errorf("materialize document %s locale %s after %d repairs: %w", publication.DocumentID, publication.Locale, repaired, err)
		}
		publication.Content = materialized
		if _, err := publicationRepo.UpsertArticlePublication(ctx, publication); err != nil {
			return fmt.Errorf("save document %s locale %s after %d repairs: %w", publication.DocumentID, publication.Locale, repaired, err)
		}
		repaired++
	}
	fmt.Printf("repair complete repaired=%d\n", repaired)
	return nil
}

func parseDocumentIDs(raw string) map[string]struct{} {
	ids := map[string]struct{}{}
	for _, item := range strings.Split(raw, ",") {
		id := strings.TrimSpace(item)
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids
}
