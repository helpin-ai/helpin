import {
  Bot,
  ClipboardCheck,
  Flag,
  MessageSquare,
  Wrench,
} from 'lucide-react';
import type {
  FlowStatus,
  FlowNodeStatus,
  FlowNodeType,
  FlowTemplateNode,
  AgentClass,
} from '@/lib/pmTypes';

// ---------------------------------------------------------------------------
// Label records
// ---------------------------------------------------------------------------

export const NODE_LABELS: Record<string, string> = {
  ensure_spec_doc: 'Ensure Spec Document',
  spec_draft: 'Draft Specification',
  spec_approval: 'Approve Specification',
  story_plan: 'Plan Stories',
  plan_approval: 'Approve Story Plan',
  create_stories: 'Create Stories',
  completion_assessment: 'Assess Completion',
  completion_review: 'Review Follow-Ups',
  create_followups: 'Create Follow-Ups',
  deal_review: 'Review Deal',
  deal_review_approval: 'Approve Deal Actions',
  apply_deal_actions: 'Apply Deal Actions',
  done: 'Complete',
};

export const TEMPLATE_LABELS: Record<string, string> = {
  'pm.epic_planning_v2': 'Epic Planning',
  'pm.story_completion_v1': 'Story Completion',
  'crm.deal_review_v1': 'Deal Review',
};

export const TARGET_LABELS: Record<string, string> = {
  epic: 'Epic',
  story: 'Story',
  crm_deal: 'CRM deal',
};

// ---------------------------------------------------------------------------
// Icon / status records
// ---------------------------------------------------------------------------

export const NODE_ICONS: Record<FlowNodeType, typeof Wrench> = {
  system_action: Wrench,
  interactive_agent: MessageSquare,
  agent_task: Bot,
  approval_gate: ClipboardCheck,
  terminal: Flag,
};

export const NODE_TYPE_LABELS: Record<FlowNodeType, string> = {
  system_action: 'System',
  interactive_agent: 'Interactive',
  agent_task: 'Autonomous',
  approval_gate: 'Approval',
  terminal: 'Terminal',
};

export const RUN_STATUS_CONFIG: Record<FlowStatus, { label: string; className: string }> = {
  running: { label: 'Running', className: 'bg-blue-500/10 text-blue-600 dark:text-blue-400' },
  awaiting_input: { label: 'Awaiting Input', className: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  awaiting_approval: { label: 'Awaiting Approval', className: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  completed: { label: 'Completed', className: 'bg-green-500/10 text-green-600 dark:text-green-400' },
  failed: { label: 'Failed', className: 'bg-red-500/10 text-red-600 dark:text-red-400' },
  cancelled: { label: 'Cancelled', className: 'bg-zinc-500/10 text-zinc-500' },
};

export const NODE_STATUS_DOT: Record<FlowNodeStatus, string> = {
  queued: 'bg-zinc-300 dark:bg-zinc-600',
  running: 'bg-blue-500',
  awaiting_input: 'bg-amber-500',
  awaiting_approval: 'bg-amber-500',
  completed: 'bg-green-500',
  failed: 'bg-red-500',
  cancelled: 'bg-zinc-400',
  skipped: 'bg-zinc-300 dark:bg-zinc-600',
};

export const ACTIVE_FLOW_STATUSES = new Set<FlowStatus>(['running', 'awaiting_input', 'awaiting_approval']);

// ---------------------------------------------------------------------------
// Label helpers
// ---------------------------------------------------------------------------

export function templateLabel(templateId: string, name?: string) {
  return name || TEMPLATE_LABELS[templateId] || templateId;
}

export function nodeLabel(nodeId: string, specLabel?: string) {
  return specLabel || NODE_LABELS[nodeId] || nodeId;
}

export function targetLabel(targetType: string) {
  return TARGET_LABELS[targetType] ?? targetType;
}

// ---------------------------------------------------------------------------
// Agent input key extraction
// ---------------------------------------------------------------------------

export interface AgentInputKeyConfig {
  key: string;
  label: string;
  required: boolean;
  agentClassFilter?: AgentClass;
  optionalHint?: string;
}

const BUILTIN_AGENT_INPUT_KEYS: Record<string, AgentInputKeyConfig[]> = {
  'pm.epic_planning_v2': [
    { key: 'spec_planner_agent_id', label: 'Spec Planner Agent', required: true, agentClassFilter: 'product_planner' },
    { key: 'story_planner_agent_id', label: 'Story Planner Agent', required: false, agentClassFilter: 'product_planner', optionalHint: 'Same as spec planner' },
  ],
  'pm.agent_story_run': [
    { key: 'agent_id', label: 'Agent', required: true },
  ],
  'pm.story_completion_v1': [
    { key: 'agent_id', label: 'Agent', required: true },
  ],
  'crm.deal_review_v1': [
    { key: 'agent_id', label: 'Agent', required: true },
  ],
};

/**
 * Extracts unique agent input keys from DB template nodes.
 * Falls back to hardcoded mapping for known builtin templates.
 */
export function extractAgentInputKeys(
  templateSlug: string,
  dbTemplateNodes?: FlowTemplateNode[],
): AgentInputKeyConfig[] {
  if (dbTemplateNodes && dbTemplateNodes.length > 0) {
    const agentNodes = dbTemplateNodes.filter(
      (n) => n.node_type === 'interactive_agent' || n.node_type === 'agent_task',
    );
    const seen = new Set<string>();
    const keys: AgentInputKeyConfig[] = [];
    for (const node of agentNodes) {
      const key = node.agent_input_key || 'agent_id';
      if (seen.has(key)) continue;
      seen.add(key);
      keys.push({
        key,
        label: formatAgentKeyLabel(key),
        required: keys.length === 0,
      });
    }
    return keys.length > 0 ? keys : [{ key: 'agent_id', label: 'Agent', required: true }];
  }

  if (BUILTIN_AGENT_INPUT_KEYS[templateSlug]) {
    return BUILTIN_AGENT_INPUT_KEYS[templateSlug];
  }

  return [{ key: 'agent_id', label: 'Agent', required: true }];
}

export function formatAgentKeyLabel(key: string): string {
  return key
    .replace(/_id$/, '')
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());
}
