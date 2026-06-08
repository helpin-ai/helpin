package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func main() {
	_ = godotenv.Load()

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	publicationRepo := repository.NewDocsHelpcenterPublicationRepository(db)
	searchRepo := repository.NewDocsHelpcenterSearchRepository(db)

	publications, err := publicationRepo.ListAllArticlePublications(ctx)
	if err != nil {
		log.Fatalf("list publications: %v", err)
	}

	for i := range publications {
		publication := publications[i]
		entries := service.BuildHelpcenterSearchEntries(publication)
		if err := searchRepo.ReplaceArticleEntries(ctx, publication.DocumentID, publication.Locale, entries); err != nil {
			log.Fatalf("reindex %s/%s: %v", publication.DocumentID, publication.Locale, err)
		}
	}

	fmt.Printf("reindexed %d help-center article publications\n", len(publications))
}
