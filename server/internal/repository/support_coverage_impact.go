package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type coverageMetricExpressions struct {
	evidence30d, evidenceAll, records30d, conversations30d, conversationsAll, customers30d, customersAll string
}

func supportCoverageMetricExpressions(dialect string) coverageMetricExpressions {
	// Missing identity is unknown, not another customer. Namespace identities to
	// avoid collisions, and never join a conversation from a different workspace.
	identity := "COALESCE('contact:' || NULLIF(sc.crm_contact_id, ''), 'email:' || NULLIF(LOWER(TRIM(sc.customer_email)), ''), 'anonymous:' || NULLIF(sc.anonymous_id, ''))"
	if dialect == "postgres" {
		identity = "COALESCE('contact:' || sc.crm_contact_id::text, 'email:' || NULLIF(LOWER(TRIM(sc.customer_email)), ''), 'anonymous:' || NULLIF(sc.anonymous_id, ''))"
	}
	count := func(value, extra string) string {
		return fmt.Sprintf("(SELECT COUNT(%s) FROM support_gap_evidence e WHERE e.gap_id = g.id AND e.workspace_id = g.workspace_id %s)", value, extra)
	}
	customers := func(extra string) string {
		return fmt.Sprintf("(SELECT COUNT(DISTINCT %s) FROM support_gap_evidence e JOIN support_conversations sc ON sc.id = e.conversation_id AND sc.workspace_id = e.workspace_id WHERE e.gap_id = g.id AND e.workspace_id = g.workspace_id %s)", identity, extra)
	}
	return coverageMetricExpressions{
		evidence30d: count("DISTINCT COALESCE(e.conversation_id, e.id)", "AND e.created_at > ?"),
		evidenceAll: count("*", ""), records30d: count("*", "AND e.created_at > ?"),
		conversations30d: count("DISTINCT e.conversation_id", "AND e.created_at > ?"),
		conversationsAll: count("DISTINCT e.conversation_id", ""),
		customers30d:     customers("AND e.created_at > ?"), customersAll: customers(""),
	}
}

func (r *SupportCoverageRepository) getGapImpactMetrics(ctx context.Context, workspaceID, gapID string) (model.SupportCoverageImpactMetrics, error) {
	m := supportCoverageMetricExpressions(r.db.Dialector.Name())
	cutoff := time.Now().AddDate(0, 0, -30)
	var result model.SupportCoverageImpactMetrics
	err := r.db.WithContext(ctx).Table("support_coverage_gaps g").
		Select(fmt.Sprintf("%s AS evidence_30d, %s AS evidence_all, %s AS evidence_records_30d, %s AS conversations_30d, %s AS conversations_all, %s AS distinct_customers_30d, %s AS distinct_customers_all", m.evidence30d, m.evidenceAll, m.records30d, m.conversations30d, m.conversationsAll, m.customers30d, m.customersAll), cutoff, cutoff, cutoff, cutoff).
		Where("g.workspace_id = ? AND g.id = ?", workspaceID, gapID).Scan(&result).Error
	if err != nil {
		return result, fmt.Errorf("get gap impact: %w", err)
	}
	result.ImpactExplanation = supportCoverageMetricsExplanation(result)
	return result, nil
}

func supportCoverageMetricsExplanation(m model.SupportCoverageImpactMetrics) string {
	parts := []string{}
	if m.Conversations30d > 0 {
		parts = append(parts, coverageCount(m.Conversations30d, "conversation", "conversations"))
		if m.DistinctCustomers30d > 0 {
			parts = append(parts, coverageCount(m.DistinctCustomers30d, "known customer", "known customers"))
		}
	} else {
		parts = append(parts, coverageCount(m.EvidenceRecords30d, "evidence record", "evidence records"))
	}
	return strings.Join(parts, " · ") + " in the last 30 days"
}

func coverageCount(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
