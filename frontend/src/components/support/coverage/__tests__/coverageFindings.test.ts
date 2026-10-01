import { describe, expect, it } from "vitest";
import {
  coverageNextStep,
  coverageStarterSuggestions,
  coverageFindings,
} from "../coverageFindings";
import type { SupportCoverageGapDetail } from "@/lib/supportCoverageTypes";

const gap = {
  status: "open",
  gap_kind: "content",
  v1_gap_type: "missing_article",
  recommendations: [
    {
      priority: "primary",
      status: "open",
      recommendation_type: "create_article",
      suggested_change: "Prepare guidance.",
    },
  ],
  suggestions: [],
} as unknown as SupportCoverageGapDetail;

describe("coverage next action", () => {
  it("points to review when an active fix is already prepared", () => {
    const prepared = {
      ...gap,
      suggestions: [{ status: "draft", is_active: true }],
    } as SupportCoverageGapDetail;
    expect(coverageNextStep(prepared)).toContain("Review");
    expect(
      coverageStarterSuggestions(prepared).map((item) => item.label),
    ).not.toContain("Prepare a fix");
  });
  it("ignores superseded drafts when choosing the next step", () => {
    const stale = {
      ...gap,
      suggestions: [{ status: "draft", is_active: false }],
    } as SupportCoverageGapDetail;
    expect(coverageNextStep(stale)).toBe("Prepare guidance.");
  });
  it("keeps verification as the next step after saving", () => {
    const saved = {
      ...gap,
      suggestions: [{ status: "applied", result_document_id: "doc-1" }],
    } as SupportCoverageGapDetail;
    expect(coverageNextStep(saved)).toContain("verify");
    expect(
      coverageStarterSuggestions(saved).map((item) => item.label),
    ).not.toContain("Prepare a fix");
  });
});

describe("saved coverage findings", () => {
  it("keeps evidence roles and all source records while limiting the visible citations", () => {
    const detail = {
      ...gap,
      confidence: 0.8,
      first_seen_at: "2026-09-20T10:00:00Z",
      last_seen_at: "2026-09-30T10:00:00Z",
      evidence_count: 50,
      evidence: [1, 2, 3].map(index => ({ id: `ev-${index}`, evidence_type: "human_reply_after_ai", conversation_id: `conv-${index}`, message_id: `msg-${index}`, sender_role: index === 1 ? "ai" : "user", excerpt: "Billing reviews corrections." })),
      related_articles: [1, 2, 3].map(index => ({ document_id: `doc-${index}`, article_title: `Guidance ${index}` })),
    } as SupportCoverageGapDetail;
    const findings = coverageFindings(detail);
    expect(findings.references).toHaveLength(4);
    expect(findings.sources).toHaveLength(6);
    expect(findings.sources?.[0].label).toContain("AI reply");
    expect(findings.sources?.[1].label).toContain("Team reply");
    expect(findings.source_summary).toContain("latest 3 of 50 evidence records");
    expect(findings.content).not.toContain("**What worked:**");
  });
  it("explains the need, cause, confirmed answer and next step with descriptive sources", () => {
    const detail = {
      ...gap,
      confidence: 0.85,
      first_seen_at: "2026-09-20T10:00:00Z",
      last_seen_at: "2026-09-30T10:00:00Z",
      evidence_count: 9,
      evidence_all: 9,
      impact_explanation: "3 conversations in the last 30 days",
      analysis_explanation: {
        customer_need: "Correct an invoice.",
        ai_failure: "The correction process is missing.",
        human_resolution: "Billing reviews requests.",
        decision_reason: "Several conversations needed a human answer.",
      },
      evidence: [{ id: "ev-1", evidence_type: "daily_conversation_analysis", conversation_id: "conv-1", excerpt: "How can I correct my invoice?", created_at: "2026-09-30T10:00:00Z" }],
      related_articles: [{ document_id: "doc-1", article_title: "Billing overview" }],
    } as SupportCoverageGapDetail;
    const findings = coverageFindings(detail);
    expect(findings.content).toContain("**Customer need:** Correct an invoice.");
    expect(findings.content).toContain("**What's missing:** The correction process is missing.");
    expect(findings.content).toContain("**What worked:** Billing reviews requests.");
    expect(findings.content).toContain("**Next step:** Prepare guidance.");
    expect(findings.references?.[0].display_title).toContain("How can I correct my invoice?");
    const bundle = findings as unknown as Record<string, unknown>;
    expect(bundle.sources).toEqual(expect.arrayContaining([expect.objectContaining({ content: "How can I correct my invoice?", reference: expect.objectContaining({ entity_id: "conv-1" }) })]));
    expect(bundle.source_summary).toBe("Showing the latest 1 of 9 evidence records.");
    expect(JSON.stringify(bundle.details)).toContain("Several conversations needed a human answer.");
  });
});
