import { describe, expect, it } from 'vitest';

import { parseArtifactPublishedPreview, parseMessagePublishedPreview, parsePublishedPreviewRawInput, resolveMessagePublishedPreview } from '../runPreviews';

describe('runPreviews', () => {
  it('parses published preview payloads from tool invocations', () => {
    const parsed = parseMessagePublishedPreview({
      content: 'Review the draft in the preview pane.',
      tool_invocations: [
        {
          tool_name: 'publish_preview',
          input: {
            panel_key: 'prd_draft',
            title: 'PRD Draft',
            format: 'markdown',
            content: '# Problem\nBody',
          },
        },
      ],
    } as never);

    expect(parsed).toEqual({
      panelKey: 'prd_draft',
      title: 'PRD Draft',
      format: 'markdown',
      content: '# Problem\nBody',
      replace: true,
      surroundingText: 'Review the draft in the preview pane.',
    });
  });

  it('parses published preview payloads from generic run preview artifacts', () => {
    const parsed = parseArtifactPublishedPreview({
      artifact_type: 'run_preview',
      inline_content: JSON.stringify({
        panel_key: 'task_plan',
        title: 'Task Plan',
        format: 'json',
        content: {
          summary: 'Slice plan',
          proposed_tasks: [],
        },
      }),
    } as never);

    expect(parsed?.panelKey).toBe('task_plan');
    expect(parsed?.format).toBe('json');
    expect(parsed?.content).toEqual({
      summary: 'Slice plan',
      proposed_tasks: [],
    });
  });

  it('parses live publish_preview tool input payloads', () => {
    const parsed = parsePublishedPreviewRawInput(JSON.stringify({
      panel_key: 'task_plan',
      title: 'Task Plan',
      format: 'json',
      content: {
        summary: 'Plan',
        proposed_tasks: [{ ref: 'story_1' }],
      },
    }));

    expect(parsed?.panelKey).toBe('task_plan');
    expect(parsed?.title).toBe('Task Plan');
  });

  it('parses json preview content when the payload content is itself a json string', () => {
    const parsed = parsePublishedPreviewRawInput(JSON.stringify({
      panel_key: 'task_plan',
      title: 'Task Plan',
      format: 'json',
      content: JSON.stringify({
        summary: 'Plan',
        proposed_tasks: [{ title: 'Task A', type: 'feature' }],
      }),
    }));

    expect(parsed?.content).toEqual({
      summary: 'Plan',
      proposed_tasks: [{ title: 'Task A', type: 'feature' }],
    });
  });

  it('parses dedicated planner preview tool payloads from tool invocations', () => {
    const parsed = parseMessagePublishedPreview({
      content: 'Review the proposed tasks.',
      tool_invocations: [
        {
          tool_name: 'publish_task_plan',
          input: {
            panel_key: 'task_plan',
            title: 'Task Plan',
            format: 'json',
            content: {
              summary: 'Slice plan',
              proposed_tasks: [{ title: 'Task A' }],
            },
          },
        },
      ],
    } as never);

    expect(parsed?.panelKey).toBe('task_plan');
    expect(parsed?.format).toBe('json');
  });

  it('parses fixed preview tools when panel metadata is implied by the tool name', () => {
    const parsed = parseMessagePublishedPreview({
      content: 'Review the current draft.',
      tool_invocations: [
        {
          tool_name: 'publish_prd_draft',
          input: {
            content: '# Problem\n\nDraft body',
          },
        },
      ],
    } as never);

    expect(parsed).toMatchObject({
      panelKey: 'prd_draft',
      title: 'PRD Draft',
      format: 'markdown',
      content: '# Problem\n\nDraft body',
    });
  });

  it('prefers assistant-linked preview artifacts over message tool invocations', () => {
    const parsed = resolveMessagePublishedPreview(
      {
        content: 'Review the draft in the preview pane.',
        sequence_no: 7,
        tool_invocations: [
          {
            tool_name: 'publish_preview',
            input: {
              panel_key: 'prd_draft',
              title: 'PRD Draft',
              format: 'markdown',
              content: '# Problem\nStale draft',
            },
          },
        ],
      } as never,
      [
        {
          artifact_type: 'run_preview',
          inline_content: JSON.stringify({
            panel_key: 'prd_draft',
            title: 'PRD Draft',
            format: 'markdown',
            content: '# Problem\nArtifact draft',
          }),
          metadata: {
            assistant_message_sequence_no: 7,
          },
        },
      ] as never,
    );

    expect(parsed?.content).toBe('# Problem\nArtifact draft');
  });
});
