import { describe, expect, it } from 'vitest';

import { buildAutonomousRuntimeStreamDisplay, getVisibleLiveTools, parseToolMessage } from '../AgentRunDrawer';
import { getAgentRunDisplayStatus } from '../agentRunConstants';
import { parseLatestCodexAuthState } from '../agentRunInteractions';

describe('AgentRunDrawer tool parsing', () => {
  it('extracts compact tool metadata from persisted tool-result blocks', () => {
    const parsed = parseToolMessage({
      id: 'msg-1',
      workspace_id: 'ws-1',
      run_id: 'run-1',
      role: 'tool',
      content: 'fallback',
      message_type: 'tool_result',
      content_blocks: [
        {
          type: 'tool_result',
          tool_name: 'request_human_input',
          output: '{"status":"paused","pause_reason":"human_input"}',
          is_error: false,
        },
      ],
      sequence_no: 1,
      created_at: new Date().toISOString(),
    });

    expect(parsed).toEqual({
      name: 'request_human_input',
      content: '{"status":"paused","pause_reason":"human_input"}',
      input: '',
      isError: false,
    });
  });

  it('keeps only unpersisted live tools visible once transcript catches up', () => {
    const visible = getVisibleLiveTools(
      [
        {
          id: 'tool-1',
          name: 'publish_preview',
          output: 'Published PRD Draft',
          status: 'completed',
        },
        {
          id: 'tool-2',
          name: 'request_human_approval',
          output: 'Awaiting approval',
          status: 'completed',
        },
        {
          id: 'tool-3',
          name: 'write_document_content',
          input: '{"document_id":"doc-1"}',
          status: 'running',
        },
      ],
      [
        {
          id: 'msg-1',
          workspace_id: 'ws-1',
          run_id: 'run-1',
          role: 'assistant',
          content: 'Draft ready.',
          message_type: 'assistant_turn',
          tool_invocations: [
            {
              tool_name: 'publish_preview',
              output_summary: 'Published PRD Draft',
            },
            {
              tool_name: 'request_human_approval',
              output_summary: 'Awaiting approval',
            },
          ],
          sequence_no: 1,
          created_at: new Date().toISOString(),
        },
      ],
    );

    expect(visible).toEqual([
      {
        id: 'tool-3',
        name: 'write_document_content',
        input: '{"document_id":"doc-1"}',
        status: 'running',
      },
    ]);
  });

  it('preserves repeated live tools until matching persisted copies exist', () => {
    const visible = getVisibleLiveTools(
      [
        {
          id: 'tool-1',
          name: 'read_document',
          output: 'Loaded doc-1',
          status: 'completed',
        },
        {
          id: 'tool-2',
          name: 'read_document',
          output: 'Loaded doc-1',
          status: 'completed',
        },
      ],
      [
        {
          id: 'msg-1',
          workspace_id: 'ws-1',
          run_id: 'run-1',
          role: 'assistant',
          content: 'Read one document.',
          message_type: 'assistant_turn',
          tool_invocations: [
            {
              tool_name: 'read_document',
              output_summary: 'Loaded doc-1',
            },
          ],
          sequence_no: 1,
          created_at: new Date().toISOString(),
        },
      ],
    );

    expect(visible).toEqual([
      {
        id: 'tool-2',
        name: 'read_document',
        output: 'Loaded doc-1',
        status: 'completed',
      },
    ]);
  });

  it('builds autonomous OpenCode transcript content from stdout/stderr artifacts', () => {
    const display = buildAutonomousRuntimeStreamDisplay(
      {
        runtime_kind: 'opencode',
        invocation_mode: 'autonomous',
        status: 'running',
      },
      [
        {
          id: 'artifact-1',
          workspace_id: 'ws-1',
          run_id: 'run-1',
          artifact_type: 'opencode_stdout',
          format: 'text',
          storage_mode: 'inline',
          inline_content: 'Step started: inspect repo\nTool: rg src\nImplemented the change.',
          metadata: {},
          sequence_no: 1,
          created_at: new Date().toISOString(),
        },
        {
          id: 'artifact-2',
          workspace_id: 'ws-1',
          run_id: 'run-1',
          artifact_type: 'opencode_stderr',
          format: 'text',
          storage_mode: 'inline',
          inline_content: 'npm warn old lockfile',
          metadata: {},
          sequence_no: 2,
          created_at: new Date().toISOString(),
        },
      ],
    );

    expect(display).toEqual({
      assistantText: '',
      processingText: 'Step started: inspect repo\nTool: rg src\nImplemented the change.',
      errorText: 'npm warn old lockfile',
    });
  });

  it('extracts Codex assistant text and activity summaries from stdout artifacts', () => {
    const display = buildAutonomousRuntimeStreamDisplay(
      {
        runtime_kind: 'codex',
        invocation_mode: 'autonomous',
        status: 'running',
      },
      [
        {
          id: 'artifact-1',
          workspace_id: 'ws-1',
          run_id: 'run-1',
          artifact_type: 'codex_stdout',
          format: 'text',
          storage_mode: 'inline',
          inline_content: [
            '{"type":"thread.started"}',
            '{"type":"item.started","item":{"type":"command_execution","command":"rg metrics"}}',
            '{"type":"item.completed","item":{"type":"agent_message","text":"Implemented the metrics update."}}',
          ].join('\n'),
          metadata: {},
          sequence_no: 1,
          created_at: new Date().toISOString(),
        },
      ],
    );

    expect(display).toEqual({
      assistantText: 'Implemented the metrics update.',
      processingText: 'Codex session started.\nRunning rg metrics',
      errorText: '',
    });
  });

  it('falls back to raw Codex stdout when no structured stream entries are present', () => {
    const display = buildAutonomousRuntimeStreamDisplay(
      {
        runtime_kind: 'codex',
        invocation_mode: 'autonomous',
        status: 'running',
      },
      [
        {
          id: 'artifact-1',
          workspace_id: 'ws-1',
          run_id: 'run-1',
          artifact_type: 'codex_stdout',
          format: 'text',
          storage_mode: 'inline',
          inline_content: 'Codex session started.\nCodex started the turn.',
          metadata: {},
          sequence_no: 1,
          created_at: new Date().toISOString(),
        },
      ],
    );

    expect(display).toEqual({
      assistantText: '',
      processingText: 'Codex session started.\nCodex started the turn.',
      errorText: '',
    });
  });

  it('parses the latest Codex auth state artifact for device-code sign-in', () => {
    const authState = parseLatestCodexAuthState([
      {
        artifact_type: 'codex_auth_state',
        inline_content: JSON.stringify({
          state: 'required',
          auth_mode: 'chatgpt_device_code',
        }),
        created_at: '2026-03-30T10:00:00Z',
      },
      {
        artifact_type: 'codex_auth_state',
        inline_content: JSON.stringify({
          state: 'pending',
          provider: 'openai',
          auth_mode: 'chatgpt_device_code',
          verification_url: 'https://chatgpt.com/device',
          user_code: 'ABCD-EFGH',
          updated_at: '2026-03-30T10:05:00Z',
        }),
        created_at: '2026-03-30T10:05:00Z',
      },
    ]);

    expect(authState).toEqual({
      state: 'pending',
      provider: 'openai',
      auth_mode: 'chatgpt_device_code',
      verification_url: 'https://chatgpt.com/device',
      user_code: 'ABCD-EFGH',
      updated_at: '2026-03-30T10:05:00Z',
    });
  });

  it('parses the latest Codex auth state artifact for browser sign-in', () => {
    const authState = parseLatestCodexAuthState([
      {
        artifact_type: 'codex_auth_state',
        inline_content: JSON.stringify({
          state: 'pending',
          provider: 'openai',
          auth_mode: 'chatgpt_device_code',
          auth_url: 'https://chatgpt.com/auth',
          updated_at: '2026-03-30T10:06:00Z',
        }),
        created_at: '2026-03-30T10:06:00Z',
      },
    ]);

    expect(authState).toEqual({
      state: 'pending',
      provider: 'openai',
      auth_mode: 'chatgpt_device_code',
      auth_url: 'https://chatgpt.com/auth',
      updated_at: '2026-03-30T10:06:00Z',
    });
  });

  it('maps authentication pauses to awaiting-auth display status', () => {
    expect(getAgentRunDisplayStatus({
      status: 'paused',
      pause_reason: 'authentication',
      approval_state: 'not_required',
    })).toBe('awaiting_auth');
  });
});
