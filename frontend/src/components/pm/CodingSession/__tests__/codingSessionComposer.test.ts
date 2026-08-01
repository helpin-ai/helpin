import { describe, expect, it } from 'vitest';

import { resolveCodingSessionComposerState } from '../codingSessionComposer';
import type { CodingSession, CodingSessionInteraction } from '@/lib/pmTypes';

function buildSession(overrides: Partial<CodingSession> = {}): CodingSession {
  return {
    id: 'run-1',
    run_id: 'run-1',
    workspace_id: 'workspace-1',
    target_type: 'document',
    target_id: 'document-1',
    agent_id: 'agent-1',
    runtime_kind: 'codex',
    invocation_mode: 'interactive',
    status: 'running',
    pause_reason: 'none',
    approval_state: 'not_required',
    title: 'Docs Operator',
    capabilities: {
      live_text_streaming: true,
      tool_streaming: true,
      repo_diff_streaming: true,
      plan_streaming: true,
      approvals: true,
      human_input: true,
      authentication: true,
      previews: true,
      terminal_output: true,
      checkpoints: true,
    },
    repo: {
      repo_name: 'helpin-ai/helpin',
      branch: 'waqar-work',
      base_branch: 'main',
      is_dirty: false,
      changed_file_count: 0,
      changed_files: [],
    },
    cached_input_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    tokens_used: 0,
    created_at: '2026-05-07T08:00:00Z',
    updated_at: '2026-05-07T08:20:00Z',
    ...overrides,
  };
}

function buildInteraction(overrides: Partial<CodingSessionInteraction> = {}): CodingSessionInteraction {
  return {
    interaction_id: 'interaction-1',
    interaction_kind: 'review_checkpoint',
    status: 'pending',
    request_schema_version: 'helpin.v1',
    request_payload: {},
    ...overrides,
  };
}

describe('resolveCodingSessionComposerState', () => {
  it('shows a disabled waiting composer while the agent is running normally', () => {
    const state = resolveCodingSessionComposerState(buildSession({ status: 'running', pause_reason: 'none' }));

    expect(state).toMatchObject({
      visible: true,
      enabled: false,
      mode: 'waiting',
      placeholder: 'Agent is working. You can answer when it asks for input.',
    });
  });

  it('enables answering only when the run is paused for human input', () => {
    const state = resolveCodingSessionComposerState(buildSession({ status: 'paused', pause_reason: 'human_input' }));

    expect(state).toMatchObject({
      visible: true,
      enabled: true,
      mode: 'answer',
      placeholder: 'Answer the agent...',
    });
  });

  it('enables chat replies when the runtime is waiting for a user message', () => {
    const state = resolveCodingSessionComposerState(buildSession({ status: 'paused', pause_reason: 'awaiting_user_message' }));

    expect(state).toMatchObject({
      visible: true,
      enabled: true,
      mode: 'answer',
      placeholder: 'Reply to continue this chat...',
    });
  });

  it('keeps authentication pauses visible but disabled', () => {
    const state = resolveCodingSessionComposerState(buildSession({ status: 'paused', pause_reason: 'authentication' }));

    expect(state).toMatchObject({
      visible: true,
      enabled: false,
      placeholder: 'Complete sign-in before responding',
    });
  });

  it('uses continuation wording for terminal recovery runs', () => {
    const state = resolveCodingSessionComposerState(buildSession({ status: 'failed', pause_reason: 'none' }));

    expect(state).toMatchObject({
      visible: true,
      enabled: true,
      mode: 'continue',
      placeholder: 'Add instructions to continue this run...',
    });
  });

  it('hides the composer when structured review controls own the response', () => {
    const state = resolveCodingSessionComposerState(
      buildSession({ status: 'paused', pause_reason: 'human_approval' }),
      buildInteraction({ interaction_kind: 'review_checkpoint' }),
    );

    expect(state.visible).toBe(false);
    expect(state.enabled).toBe(false);
  });

  it('hides the composer when the run completed successfully', () => {
    const state = resolveCodingSessionComposerState(buildSession({ status: 'completed', pause_reason: 'none' }));

    expect(state.visible).toBe(false);
  });
});
