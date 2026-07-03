import type { Agent } from './pmTypes';

const FLOW_TEMPLATE_NAMES: Record<string, string> = {
  release_notes_writer: 'Release Notes Writer',
  competitive_intelligence_digest: 'Competitors Changelog Tracking Report',
  competitors_changelog_tracking_report: 'Competitors Changelog Tracking Report',
  dependency_auditor: 'Dependency Auditor',
  security_triage: 'Security Triage',
  review_merged_prs: 'Review merged PRs',
  triage_failing_checks: 'Triage failing checks',
  buying_signal_to_task: 'High-intent buyer signal to task',
  stale_task_escalation: 'Stale task escalation',
  docs_freshness_sweep: 'Docs freshness sweep',
};

export function agentTemplateSourceLabel(agent: Pick<Agent, 'template_key' | 'source_template_key'>) {
  const key = agent.template_key || agent.source_template_key;
  if (!key) return null;
  const name = FLOW_TEMPLATE_NAMES[key] ?? humanizeTemplateKey(key);
  return `from ${name} template`;
}

function humanizeTemplateKey(key: string) {
  return key
    .split('_')
    .filter(Boolean)
    .map((word) => word.slice(0, 1).toUpperCase() + word.slice(1))
    .join(' ');
}
