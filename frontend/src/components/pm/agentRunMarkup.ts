import type { OrchestrationProposal } from '@/lib/pmTypes';

const specDraftRegex = /<spec_draft>\s*([\s\S]*?)\s*<\/spec_draft>/i;
const storyPlanRegex = /<story_plan>\s*([\s\S]*?)\s*<\/story_plan>/i;
const approvalRequestRegex = /<approval_request\b([^>]*?)(?:>([\s\S]*?)<\/approval_request>|\/\s*>)/i;
const approvalPhaseAttrRegex = /\bphase\s*=\s*["']([^"']+)["']/i;
const approvalTitleAttrRegex = /\btitle\s*=\s*["']([^"']+)["']/i;
const approvalSummaryAttrRegex = /\bsummary\s*=\s*["']([^"']+)["']/i;
const approvalTitleRegex = /<title>\s*([\s\S]*?)\s*<\/title>/i;
const approvalSummaryRegex = /<summary>\s*([\s\S]*?)\s*<\/summary>/i;

export interface ParsedSpecDraft {
  draft: string;
  surroundingText: string;
}

export interface ParsedStoryPlan {
  plan: OrchestrationProposal;
  raw: string;
  surroundingText: string;
}

export interface ParsedApprovalRequest {
  phase: string;
  title: string;
  summary: string;
  surroundingText: string;
}

export function parseSpecDraft(content: string): ParsedSpecDraft | null {
  const match = specDraftRegex.exec(content);
  if (!match) return null;
  return {
    draft: match[1].trim(),
    surroundingText: content.replace(specDraftRegex, '').trim(),
  };
}

export function parseStoryPlan(content: string): ParsedStoryPlan | null {
  const match = storyPlanRegex.exec(content);
  if (!match) return null;

  const raw = match[1].trim();
  try {
    const plan = JSON.parse(raw) as OrchestrationProposal;
    return {
      plan,
      raw,
      surroundingText: content.replace(storyPlanRegex, '').trim(),
    };
  } catch {
    return null;
  }
}

export function parseApprovalRequest(content: string): ParsedApprovalRequest | null {
  const match = approvalRequestRegex.exec(content);
  if (!match) return null;

  const attrs = match[1] ?? '';
  const body = (match[2] ?? '').trim();
  const phaseMatch = approvalPhaseAttrRegex.exec(attrs);
  const phase = phaseMatch?.[1]?.trim().toLowerCase();
  if (!phase) return null;

  const title = approvalTitleRegex.exec(body)?.[1]?.trim()
    ?? approvalTitleAttrRegex.exec(attrs)?.[1]?.trim()
    ?? body;
  const summary = approvalSummaryRegex.exec(body)?.[1]?.trim()
    ?? approvalSummaryAttrRegex.exec(attrs)?.[1]?.trim()
    ?? '';

  return {
    phase,
    title,
    summary,
    surroundingText: content.replace(approvalRequestRegex, '').trim(),
  };
}
