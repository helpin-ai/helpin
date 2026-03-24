import { describe, expect, it } from 'vitest';

import { parseArtifactPublishedPreview, parseMessagePublishedPreview, parsePublishedPreviewRawInput } from '../agentRunPreviews';

describe('agentRunPreviews', () => {
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
        panel_key: 'story_plan',
        title: 'Story Plan',
        format: 'json',
        content: {
          summary: 'Slice plan',
          proposed_stories: [],
        },
      }),
    } as never);

    expect(parsed?.panelKey).toBe('story_plan');
    expect(parsed?.format).toBe('json');
    expect(parsed?.content).toEqual({
      summary: 'Slice plan',
      proposed_stories: [],
    });
  });

  it('parses live publish_preview tool input payloads', () => {
    const parsed = parsePublishedPreviewRawInput(JSON.stringify({
      panel_key: 'story_plan',
      title: 'Story Plan',
      format: 'json',
      content: {
        summary: 'Plan',
        proposed_stories: [{ ref: 'story_1' }],
      },
    }));

    expect(parsed?.panelKey).toBe('story_plan');
    expect(parsed?.title).toBe('Story Plan');
  });
});
