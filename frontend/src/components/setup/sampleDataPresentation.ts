import type { SampleDataEntity, SampleDataStatus } from '@/lib/sampleDataTypes';
import type { WorkspaceModule } from '@/lib/types';

const entityLabels: Array<[SampleDataEntity, string, string]> = [
  ['support_conversation', 'conversation', 'conversations'],
  ['docs_document', 'help article', 'help articles'],
  ['pm_epic', 'project', 'projects'],
  ['pm_task', 'task', 'tasks'],
  ['crm_company', 'company', 'companies'],
  ['crm_contact', 'contact', 'contacts'],
  ['crm_deal', 'deal', 'deals'],
  ['automation_rule', 'Flow (turned off)', 'Flows (turned off)'],
];

const sampleModuleOrder = ['support', 'docs', 'pm', 'crm', 'automation'] as const;

const moduleContent: Partial<Record<WorkspaceModule, string>> = {
  support: 'support conversations',
  docs: 'help articles',
  pm: 'a project with tasks',
  crm: 'CRM companies, contacts and deals',
  automation: 'example Flows that stay off until you turn them on',
};

const containerLabels: Partial<Record<SampleDataEntity, string>> = {
  docs_space: 'the sample help center space',
  workspace_team: 'the sample team',
  crm_pipeline: 'the sample sales pipeline',
};

/** "4 conversations, 3 help articles, 1 project, …" in a fixed, readable order. */
export function summarizeSampleCounts(status: Pick<SampleDataStatus, 'counts'>): string {
  return entityLabels
    .map(([entity, singular, plural]) => {
      const count = status.counts[entity] ?? 0;
      return count > 0 ? `${count} ${count === 1 ? singular : plural}` : '';
    })
    .filter(Boolean)
    .join(', ');
}

/** What loading adds, limited to the modules available to the viewer. */
export function describeSampleModules(modules: WorkspaceModule[]): string {
  const parts = sampleModuleOrder
    .filter((module) => modules.includes(module))
    .map((module) => moduleContent[module] ?? module);
  if (parts.length <= 1) return parts.join('');
  return `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}`;
}

/** Confirmation text after removal, naming any sample records that were kept. */
export function sampleRemovalMessage(status: SampleDataStatus): string {
  const retained = Object.entries(status.retained ?? {}).filter(([, count]) => (count ?? 0) > 0);
  const kept = retained
    .map(([entity]) => containerLabels[entity as SampleDataEntity])
    .filter(Boolean);
  const keptFlows = retained.some(([entity]) => entity === 'automation_rule');
  const sentences = ['Sample data removed.'];
  if (kept.length > 0) sentences.push(`Kept ${kept.join(' and ')} because it now holds your own records.`);
  if (keptFlows) sentences.push('Kept the sample Flows you turned on, with their run history.');
  return sentences.join(' ');
}
