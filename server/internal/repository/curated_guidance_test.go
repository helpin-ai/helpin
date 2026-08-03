package repository

import (
	"strings"
	"testing"
)

func TestCuratedGuidancePostgresVectorOrderUsesRankedProjection(t *testing.T) {
	sql := curatedGuidancePostgresSearchSQL(
		"CASE WHEN embedding IS NULL THEN 0 ELSE 1 END AS vector_score",
		"AND language = ''",
		"",
		"(lexical_score + vector_score) DESC",
	)

	rankedFrom := strings.Index(sql, "FROM ranked")
	vectorOrder := strings.Index(sql, "ORDER BY (lexical_score + vector_score) DESC")
	if !strings.Contains(sql, "WITH ranked AS") || rankedFrom < 0 || vectorOrder < rankedFrom {
		t.Fatalf("vector aliases must be ordered in the outer ranked query:\n%s", sql)
	}
	if !strings.Contains(sql, "language, updated_at,") {
		t.Fatalf("ranked projection must retain updated_at for deterministic ordering:\n%s", sql)
	}
}
