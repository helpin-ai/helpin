// @vitest-environment jsdom

import React, { act } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { draftFromRule, FlowComposer, serializeDraft, validateDraft } from '../AutomationFlows';
import { AgentRunNowDialog } from '../agents/AgentRunNowDialog';
import { buildAutomationFlowsPath, FLOW_TARGET_MODES } from '@/lib/automationUi';
import { CRM_AGENT_TARGET_OPTIONS } from '@/lib/agentCRMTargets';
import type { Agent, AutomationRule } from '@/lib/pmTypes';

vi.mock('@/components/automation/CRMRecordPicker', () => ({
  CRMRecordPicker: ({ workspaceId, targetType, value, disabled, onChange }: { workspaceId: string; targetType: string; value: string; disabled?: boolean; onChange: (id: string) => void }) => (
    <button data-testid="crm-picker" data-workspace={workspaceId} data-type={targetType} data-value={value} disabled={disabled} onClick={() => onChange('record-2')}>Choose CRM record</button>
  ),
}));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const baseRule: AutomationRule = {
  id: 'flow-1', workspace_id: 'ws-1', name: 'Review renewal', enabled: false,
  trigger_type: 'cron', trigger_config: { schedule: '0 9 * * *' },
  action_type: 'start_agent_run', action_config: { agent_id: 'agent-1', target_type: 'crm_deal', target_id: 'record-1' },
  position: 0, stop_on_match: false, created_at: '2026-09-06T09:00:00Z', updated_at: '2026-09-06T09:00:00Z',
};
let container: HTMLDivElement | undefined;
let root: Root | undefined;
let queryClient: QueryClient;

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = undefined;
  queryClient?.clear();
});

function render(node: React.ReactNode) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  queryClient.setQueryData(['ai-profiles', 'ws-1'], []);
  act(() => root?.render(<QueryClientProvider client={queryClient}>{node}</QueryClientProvider>));
}

describe('CRM records in existing automation builders', () => {
  it.each(CRM_AGENT_TARGET_OPTIONS)('preserves $label when opening and saving an existing Flow', ({ value }) => {
    const rule = { ...baseRule, action_config: { ...baseRule.action_config, target_type: value } };
    const draft = draftFromRule(rule, [], 'UTC');
    expect(draft.targetMode).toBe(value);
    expect(validateDraft(draft)).toBeNull();
    expect(serializeDraft(draft, 'ws-1', 'UTC').action_config).toEqual(rule.action_config);
    expect(validateDraft({ ...draft, targetId: '' })).toBe('Choose a target');
    expect(FLOW_TARGET_MODES.has(value)).toBe(true);
    expect(buildAutomationFlowsPath('acme', { target_mode: value, target_id: 'record-1' })).toContain(`target_mode=${value}&target_id=record-1`);
    expect(rule.enabled).toBe(false);
  });

  it.each(['task', 'epic', 'repository', 'workspace'])('preserves the existing %s Flow target', (type) => {
    const rule = { ...baseRule, action_config: { agent_id: 'agent-1', target_type: type, target_id: type === 'workspace' ? 'ws-1' : 'record-1' } };
    expect(serializeDraft(draftFromRule(rule, [], 'UTC'), 'ws-1', 'UTC').action_config).toEqual(rule.action_config);
  });

  it('uses a CRM picker in the Flow composer without changing the current record or agent', () => {
    const draft = draftFromRule({ ...baseRule, action_config: { agent_id: 'agent-1', target_type: 'crm_company', target_id: 'record-1' } }, [], 'UTC');
    const onDraftChange = vi.fn();
    render(<FlowComposer
      workspaceId="ws-1" open mode="edit" draft={draft}
      workflows={[]} statesById={new Map()} agents={[{ id: 'agent-1', name: 'Sales assistant', allowed_targets: ['crm_company'], team_ids: [] } as unknown as Agent]}
      accessibleTeamIds={new Set()} canSeeAllAgents tasks={[]} epics={[]} repositories={[]}
      timezone="UTC" saving={false} canEdit onOpenChange={vi.fn()} onDraftChange={onDraftChange} onSave={vi.fn()}
    />);
    expect(document.body.textContent).toContain('a specific company');
    const picker = document.querySelector('[data-testid="crm-picker"]');
    expect(picker?.getAttribute('data-type')).toBe('crm_company');
    expect(picker?.getAttribute('data-workspace')).toBe('ws-1');
    expect(picker?.getAttribute('data-value')).toBe('record-1');
    expect(onDraftChange).not.toHaveBeenCalled();
    act(() => (picker as HTMLElement).click());
    const updated = onDraftChange.mock.calls[0][0](draft);
    expect(updated).toMatchObject({ targetMode: 'crm_company', targetId: 'record-2', agentId: 'agent-1' });
  });

  it.each(CRM_AGENT_TARGET_OPTIONS)('uses record search, not a raw ID input, for Run now on $label', ({ value, label }) => {
    const onTargetIdChange = vi.fn();
    render(<AgentRunNowDialog
      aiConnection={{}} onAIConnectionChange={vi.fn()} deliveryMode="preview" onDeliveryModeChange={vi.fn()}
      open onOpenChange={vi.fn()} agent={null} targets={[value]} targetLabel={() => label}
      targetType={value} onTargetTypeChange={vi.fn()} workspaceName="Acme" workspaceId="ws-1"
      repositories={[]} runnableRepositories={[]} repositoriesLoading={false}
      targetId="" onTargetIdChange={onTargetIdChange} baseBranch="" onBaseBranchChange={vi.fn()}
      additionalContext="" onAdditionalContextChange={vi.fn()} submitting={false} canSubmit={false} onSubmit={vi.fn()}
    />);
    expect(document.querySelector('#run-now-target-id')).toBeNull();
    const picker = document.querySelector('[data-testid="crm-picker"]');
    expect(picker?.getAttribute('data-type')).toBe(value);
    act(() => (picker as HTMLElement).click());
    expect(onTargetIdChange).toHaveBeenCalledTimes(1);
    expect(onTargetIdChange).toHaveBeenCalledWith('record-2');
  });
});
