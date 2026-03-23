import { describe, expect, it } from 'vitest';

import { getVisibleLiveTools, parseToolMessage } from '../AgentRunDrawer';

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
          output: '{"status":"awaiting_input"}',
          is_error: false,
        },
      ],
      sequence_no: 1,
      created_at: new Date().toISOString(),
    });

    expect(parsed).toEqual({
      name: 'request_human_input',
      content: '{"status":"awaiting_input"}',
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
});
