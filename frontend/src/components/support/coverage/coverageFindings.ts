import type { DockContextMessage, DockContextSource, DockEntityReference } from "@/lib/dockTypes";
import type { SupportCoverageGapDetail } from "@/lib/supportCoverageTypes";
import {
  starterSuggestionsForContext,
  type StarterSuggestion,
} from "@/components/agents/dock/starterSuggestions";
import { coverageConfidenceLabel, coverageDiagnosis, evidenceTypeLabel, formatCoverageImpact } from "./coverageUi";

function preparedFixState(gap: SupportCoverageGapDetail) {
  if (
    gap.suggestions.some(
      (suggestion) =>
        suggestion.status === "draft" &&
        suggestion.is_active !== false &&
        !suggestion.superseded_at,
    )
  )
    return "draft";
  if (
    !gap.recurrence_reopened &&
    gap.suggestions.some(
      (suggestion) =>
        suggestion.status === "applied" && suggestion.result_document_id,
    )
  )
    return "saved";
  return null;
}

export function coverageStarterSuggestions(
  gap: SupportCoverageGapDetail,
): StarterSuggestion[] {
  if (preparedFixState(gap))
    return [
      {
        label: "Check the draft",
        prompt:
          "Review the existing prepared fix for this gap against the source conversations and documents. Flag missing facts or inaccurate guidance, and link the draft. Keep it unpublished for review.",
      },
      {
        label: "Plan verification",
        prompt:
          "Explain how we should verify that this prepared fix lets the AI handle this customer need. Identify any remaining retrieval, policy, data, or action blockers.",
      },
    ];
  return starterSuggestionsForContext("support_coverage_gap");
}

export function coverageNextStep(gap: SupportCoverageGapDetail): string {
  const prepared = preparedFixState(gap);
  if (prepared === "draft")
    return "Review the proposed fix, then verify that the AI can use the approved guidance.";
  if (prepared === "saved")
    return "Review and publish the saved draft, then verify that the AI can use the fix before marking this gap resolved.";
  const recommendation = gap.recommendations.find(
    (rec) =>
      rec.priority === "primary" && ["open", "draft"].includes(rec.status),
  );
  if (recommendation?.suggested_change) return recommendation.suggested_change;
  if (gap.gap_kind === "data")
    return "Identify the missing account data and how the AI can access it.";
  if (gap.gap_kind === "action")
    return "Check the missing action and prepare the guidance or owner handoff needed to resolve it.";
  if (gap.gap_kind === "policy")
    return "Agree on the policy or escalation rule, then prepare clear guidance.";
  if (gap.failure_mode === "no_retrieval")
    return "Check why the AI cannot find the existing guidance.";
  if (gap.v1_gap_type === "weak_article")
    return "Prepare the missing guidance for the existing article.";
  return "Review the sources and prepare the right fix for review.";
}

function sourceTitle(excerpt: string, fallback: string): string {
  const text = excerpt.replace(/[#*_`>]/g, "").replace(/\s+/g, " ").trim();
  return text ? (text.length > 90 ? `${text.slice(0, 87).trimEnd()}…` : text) : fallback;
}

/** Fast local preview; after first send the server's complete snapshot is authoritative. */
export function coverageFindings(gap: SupportCoverageGapDetail): DockContextMessage {
  const articles = new Map(gap.related_articles.map(article => [article.document_id, article.article_title]));
  const sources: DockContextSource[] = gap.evidence.map((evidence, index) => ({
    id: evidence.id,
    label: evidence.message_id
      ? ({ ai: "AI reply", user: "Team reply", agent: "Team reply", customer: "Customer message", visitor: "Customer message" }[evidence.sender_role] || "Message")
      : evidenceTypeLabel(evidence.evidence_type),
    content: evidence.excerpt,
    captured_at: evidence.created_at,
    reference: evidence.conversation_id ? {
      entity_type: "support_conversation",
      entity_id: evidence.conversation_id,
      display_title: sourceTitle(evidence.excerpt, `Conversation ${index + 1}`),
    } : evidence.document_id ? {
      entity_type: "document",
      entity_id: evidence.document_id,
      display_title: articles.get(evidence.document_id) || "Source article",
    } : undefined,
  }));
  const seenDocuments = new Set(sources.filter(source => source.reference?.entity_type === "document").map(source => source.reference!.entity_id));
  for (const article of gap.related_articles) {
    if (seenDocuments.has(article.document_id)) continue;
    seenDocuments.add(article.document_id);
    sources.push({
      id: `document:${article.document_id}`,
      label: "Related guidance",
      reference: { entity_type: "document", entity_id: article.document_id, display_title: article.article_title || "Untitled article" },
    });
  }
  const references: DockEntityReference[] = [];
  const seen = new Set<string>();
  for (const type of ["support_conversation", "document"]) {
    let count = 0;
    for (const source of sources) {
      const reference = source.reference;
      if (!reference || reference.entity_type !== type || seen.has(`${type}:${reference.entity_id}`)) continue;
      seen.add(`${type}:${reference.entity_id}`);
      references.push(reference);
      if (++count === 2) break;
    }
  }
  const nextStep = coverageNextStep(gap);
  const explanation = gap.analysis_explanation;
  const analysis = [
    explanation?.decision_reason?.trim(),
    `${coverageConfidenceLabel(gap.confidence).text} · First seen ${gap.first_seen_at.slice(0, 10)} · Last seen ${gap.last_seen_at.slice(0, 10)}`,
    ...gap.recommendations.map(rec => [
      `**${rec.target_title || rec.recommendation_type.replaceAll("_", " ")}${rec.priority === "primary" ? " · recommended" : ""}**`,
      rec.rationale,
      rec.suggested_change !== nextStep ? rec.suggested_change : null,
      rec.implementation_notes,
      rec.target_url ? `[Open target](${rec.target_url})` : null,
    ].filter(Boolean).join("\n\n")),
  ].filter(Boolean).join("\n\n");
  const total = gap.evidence_all ?? gap.evidence_count;
  return {
    content: [
      `**Customer need:** ${explanation?.customer_need?.trim() || "Review the sources to confirm what customers need."}`,
      `**What's missing:** ${explanation?.ai_failure?.trim() || coverageDiagnosis(gap) || "The evidence needs review to confirm what is missing."}`,
      explanation?.human_resolution?.trim() ? `**What worked:** ${explanation.human_resolution.trim()}` : null,
      `**Next step:** ${nextStep}`,
      `${formatCoverageImpact(gap)}.`,
      gap.confidence < 0.6 ? "The cause is uncertain. Confirm it with the sources before applying a fix." : null,
    ].filter(Boolean).join("\n\n"),
    captured_at: gap.last_seen_at,
    references,
    sources,
    source_summary: gap.evidence.length < total ? `Showing the latest ${gap.evidence.length} of ${total} evidence records.` : gap.evidence.length === 0 ? "No source excerpts are available for this gap." : undefined,
    details: [{ label: "Analysis details", content: analysis }],
  };
}
