import { describe, expect, it } from 'vitest';
import {
  applyCustomAgentDraftToForm,
  buildCustomAgentCreatePayload,
  createDefaultCustomAgentForm,
  defaultModelForAgentProvider,
  summarizeCustomAgentCreate,
  validateCustomAgentCreateForm,
} from '../customAgentCreateModel';

describe('custom agent create model', () => {
  it('keeps CRM selections explicit and summarizes them using familiar names', () => {
    const form = {
      ...createDefaultCustomAgentForm(),
      name: 'Sales assistant',
      allowed_targets: ['crm_contact', 'crm_company'] as const,
    };
    const selected = { ...form, allowed_targets: [...form.allowed_targets] };
    expect(buildCustomAgentCreatePayload('workspace-1', selected, false).allowed_targets).toEqual(['crm_contact', 'crm_company']);
    expect(summarizeCustomAgentCreate(selected)).toContain('CRM contacts and CRM companies');
    expect(createDefaultCustomAgentForm().allowed_targets).toEqual(['task']);
  });

  it('uses a public model-size default without exposing a technical route', () => {
    const form = createDefaultCustomAgentForm();

    expect(form.name).toBe('');
    expect(form.icon_key).toBe('violet_star');
    expect(form.runtime_kind).toBe('native_sdk');
    expect(form.default_invocation_mode).toBe('interactive');
    expect(form.supported_modes).toEqual(['autonomous', 'interactive']);
    expect(form.allowed_targets).toEqual(['task']);
    expect(form.allowed_tools).toEqual([]);
    expect(form.approval_mode).toBe('mutating_tools');
    expect(form.max_concurrent_runs).toBe('1');
    expect(form.preset_key).toBe('code_builder');
    expect(form.preset_version_key).toBe('code_builder_default');
    expect(form.model_tier).toBe('large');
    expect(form.instruction_preamble).toBe('');
    expect(form.instruction_skills).toEqual([]);
    expect(form.max_tool_steps).toBe('');
  });

  it('applies a validated backend draft to the custom form', () => {
    const form = createDefaultCustomAgentForm();
    const updated = applyCustomAgentDraftToForm(form, {
      name: 'Support Reply Assistant',
      role: 'Custom Agent',
      system_prompt: 'Draft support replies.',
      allowed_targets: ['support_conversation'],
      allowed_tools: ['search_documents'],
      skills: [{ key: 'support_style' }],
      approval_mode: 'mutating_tools',
      model_tier: 'medium',
      default_invocation_mode: 'interactive',
      max_concurrent_runs: 1,
    });

    expect(updated).toMatchObject({
      name: 'Support Reply Assistant',
      system_prompt: 'Draft support replies.',
      allowed_targets: ['support_conversation'],
      allowed_tools: ['search_documents'],
      skills: [{ key: 'support_style' }],
      approval_mode: 'mutating_tools',
      model_tier: 'medium',
      default_invocation_mode: 'interactive',
      max_concurrent_runs: '1',
    });
  });

  it('builds the existing automation create-agent payload without template fields', () => {
    const form = {
      ...createDefaultCustomAgentForm(),
      name: 'Support Helper',
      system_prompt: 'Help triage support conversations.',
      allowed_targets: ['support_conversation' as const],
      teamAccessMode: 'specific_teams' as const,
      team_ids: ['team-1', 'team-2'],
    };

    const payload = buildCustomAgentCreatePayload('workspace-1', form, false);

    expect(payload).toMatchObject({
      workspace_id: 'workspace-1',
      name: 'Support Helper',
      icon_key: 'violet_star',
      model_tier: 'large',
      system_prompt: 'Help triage support conversations.',
      trigger_mode: 'manual',
      team_ids: ['team-1', 'team-2'],
      allowed_targets: ['support_conversation'],
      approval_mode: 'mutating_tools',
      max_concurrent_runs: 1,
      default_invocation_mode: 'interactive',
    });
    expect(payload).not.toHaveProperty('role');
    expect(payload).not.toHaveProperty('team_id');
    expect(payload).not.toHaveProperty('preset_key');
    expect(payload).not.toHaveProperty('preset_version_key');
    expect(payload).not.toHaveProperty('instruction_preamble');
    expect(payload).not.toHaveProperty('instruction_skills');
    expect(payload).not.toHaveProperty('create_flow');
    expect(payload).not.toHaveProperty('flow');
    expect(payload).not.toHaveProperty('overrides');
    expect(payload).not.toHaveProperty('provider');
    expect(payload).not.toHaveProperty('model');
    expect(payload).not.toHaveProperty('runtime_kind');
    expect(payload).not.toHaveProperty('execution_config');
  });

  it('uses explicit provider defaults for model routing', () => {
    expect(defaultModelForAgentProvider('openai')).toBe('gpt-5.6-terra');
    expect(defaultModelForAgentProvider('openrouter')).toBe('openai/gpt-5.6-terra');
    expect(defaultModelForAgentProvider('anthropic')).toBe('claude-opus-4-8');
    expect(defaultModelForAgentProvider('openai', [{
      value: 'openai',
      label: 'OpenAI',
      default_model: 'gpt-server-default',
      model_placeholder: 'gpt-server-default',
      supports_reasoning_effort: true,
      supports_service_tier: true,
    }])).toBe('gpt-server-default');
  });

  it('pins current optional field behavior for empty team and skills', () => {
    const empty = { ...createDefaultCustomAgentForm(), name: 'Agent' };

    const emptyPayload = buildCustomAgentCreatePayload('workspace-1', empty, false);
    expect(emptyPayload.team_ids).toEqual([]);
    expect(emptyPayload).not.toHaveProperty('team_id');
    expect(emptyPayload.skills).toBeUndefined();

    const skill = { key: 'release_notes' };
    const withSkill = { ...empty, skills: [skill] };
    expect(buildCustomAgentCreatePayload('workspace-1', withSkill, false).skills).toEqual([skill]);
  });

  it('normalizes invalid concurrency values to one', () => {
    const base = { ...createDefaultCustomAgentForm(), name: 'Agent' };

    expect(buildCustomAgentCreatePayload('workspace-1', { ...base, max_concurrent_runs: '' }, false).max_concurrent_runs).toBe(1);
    expect(buildCustomAgentCreatePayload('workspace-1', { ...base, max_concurrent_runs: '0' }, false).max_concurrent_runs).toBe(1);
    expect(buildCustomAgentCreatePayload('workspace-1', { ...base, max_concurrent_runs: 'abc' }, false).max_concurrent_runs).toBe(1);
    expect(buildCustomAgentCreatePayload('workspace-1', { ...base, max_concurrent_runs: '3' }, false).max_concurrent_runs).toBe(3);
  });

  it('does not submit hidden runtime execution controls', () => {
    const nativeForm = {
      ...createDefaultCustomAgentForm(),
      name: 'Native agent',
      runtime_kind: 'native_sdk' as const,
      max_tool_steps: '640',
    };

    const payload = buildCustomAgentCreatePayload('workspace-1', nativeForm, true);
    expect(payload).not.toHaveProperty('execution_config');
    expect(payload).not.toHaveProperty('runtime_kind');
    expect(payload).not.toHaveProperty('provider');
    expect(payload).not.toHaveProperty('model');

    const codexForm = {
      ...nativeForm,
      runtime_kind: 'codex' as const,
    };
    expect(buildCustomAgentCreatePayload('workspace-1', codexForm, true).model_tier).toBe('large');
  });

  it('keeps technical routing hidden regardless of advanced panel state', () => {
    const base = { ...createDefaultCustomAgentForm(), name: 'Agent' };

    expect(buildCustomAgentCreatePayload('workspace-1', base, false)).not.toHaveProperty('runtime_kind');
    expect(buildCustomAgentCreatePayload('workspace-1', base, false)).not.toHaveProperty('monthly_token_budget');

    const advancedClosedOpenCode = { ...base, runtime_kind: 'opencode' as const };
    expect(buildCustomAgentCreatePayload('workspace-1', advancedClosedOpenCode, false)).not.toHaveProperty('runtime_kind');

    const advancedOpen = { ...base, monthly_token_budget: '5000' };
    expect(buildCustomAgentCreatePayload('workspace-1', advancedOpen, true)).toMatchObject({ model_tier: 'large' });
    expect(buildCustomAgentCreatePayload('workspace-1', advancedOpen, true)).not.toHaveProperty('monthly_token_budget');
  });

  it('validates only fields required by the custom create path', () => {
    const form = createDefaultCustomAgentForm();

    expect(validateCustomAgentCreateForm(form)).toEqual(['agent name', 'instructions', 'tool or skill']);
    expect(validateCustomAgentCreateForm({
      ...form,
      name: 'Planner',
      system_prompt: 'Plan work clearly.',
      allowed_tools: ['list_tasks'],
      allowed_targets: [],
    })).toEqual(['working area']);
    expect(validateCustomAgentCreateForm({
      ...form,
      name: 'Planner',
      system_prompt: 'Plan work clearly.',
      allowed_tools: ['list_tasks'],
      teamAccessMode: 'specific_teams',
      team_ids: [],
    })).toEqual(['team access']);
    expect(validateCustomAgentCreateForm({
      ...form,
      name: 'Planner',
      system_prompt: 'Plan work clearly.',
      allowed_tools: ['list_tasks'],
    })).toEqual([]);
  });

  it('summarizes without runtime provider model or manual trigger jargon', () => {
    const form = { ...createDefaultCustomAgentForm(), name: 'Planning Helper', allowed_targets: ['task', 'epic'] as const };

    const summary = summarizeCustomAgentCreate(form, 'Product');

    expect(summary).toContain('Create Planning Helper');
    expect(summary).toContain('Product');
    expect(summary).toContain('tasks and epics');
    expect(summary).toContain('interactive');
    expect(summary).not.toContain('manual');
    expect(summary).not.toContain('opencode');
    expect(summary).not.toContain('Anthropic');
  });

  it('summarizes sprint and objective working areas', () => {
    const form = {
      ...createDefaultCustomAgentForm(),
      name: 'Outcome Planner',
      allowed_targets: ['sprint', 'objective'] as const,
    };

    expect(summarizeCustomAgentCreate(form)).toContain('sprints and objectives');
  });
});
