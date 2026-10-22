//go:build integration

package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupCoverageAnalysisPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("COVERAGE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set COVERAGE_TEST_POSTGRES_DSN to a disposable PostgreSQL database with pgvector")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil {
			t.Error(err)
		}
	})
	schema := "coverage_test_" + uuid.NewString()[:8]
	for _, statement := range []string{
		"CREATE EXTENSION IF NOT EXISTS vector",
		"CREATE SCHEMA " + schema,
		"SET LOCAL search_path TO " + schema + ", public",
	} {
		if err := tx.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.AutoMigrate(&model.SupportCoverageConversationAnalysis{}); err != nil {
		t.Fatal(err)
	}
	return tx
}

func TestRecordConversationAnalysisPostgresLeavesMissingEmbeddingNull(t *testing.T) {
	db := setupCoverageAnalysisPostgres(t)
	repo := NewSupportCoverageAnalysisRepository(db)
	analysis := &model.SupportCoverageConversationAnalysis{
		WorkspaceID: uuid.NewString(), RunID: uuid.NewString(), ConversationID: uuid.NewString(),
		TranscriptHash: "hash", AnalyzerVersion: "v4", Status: model.SupportCoverageConversationAnalysisStatusFailed,
	}

	if err := repo.RecordConversationAnalysis(context.Background(), analysis); err != nil {
		t.Fatalf("record analysis without embedding: %v", err)
	}
	var nullEmbedding bool
	if err := db.Raw("SELECT embedding IS NULL FROM support_coverage_conversation_analyses WHERE id = ?", analysis.ID).Scan(&nullEmbedding).Error; err != nil {
		t.Fatal(err)
	}
	if !nullEmbedding {
		t.Fatal("missing embedding should be stored as NULL")
	}
}

func TestRecordConversationAnalysisPostgresKeepsProvidedEmbedding(t *testing.T) {
	db := setupCoverageAnalysisPostgres(t)
	repo := NewSupportCoverageAnalysisRepository(db)
	values := make([]string, 1536)
	for i := range values {
		values[i] = "0.01"
	}
	analysis := &model.SupportCoverageConversationAnalysis{
		WorkspaceID: uuid.NewString(), RunID: uuid.NewString(), ConversationID: uuid.NewString(),
		TranscriptHash: "hash", AnalyzerVersion: "v4", Embedding: "[" + strings.Join(values, ",") + "]",
	}

	if err := repo.RecordConversationAnalysis(context.Background(), analysis); err != nil {
		t.Fatalf("record analysis with embedding: %v", err)
	}
	var dimensions int
	if err := db.Raw("SELECT vector_dims(embedding) FROM support_coverage_conversation_analyses WHERE id = ?", analysis.ID).Scan(&dimensions).Error; err != nil {
		t.Fatal(err)
	}
	if dimensions != 1536 {
		t.Fatalf("embedding dimensions = %d, want 1536", dimensions)
	}
}
