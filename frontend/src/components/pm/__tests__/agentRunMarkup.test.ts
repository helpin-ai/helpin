import { describe, expect, it } from 'vitest';

import { parseApprovalRequest, parseSpecDraft, parseStoryPlan } from '../agentRunMarkup';

describe('agentRunMarkup', () => {
  it('parses a spec draft block and preserves surrounding text', () => {
    const parsed = parseSpecDraft('Intro\n<spec_draft>\n# PRD\nBody\n</spec_draft>\nOutro');

    expect(parsed).toEqual({
      draft: '# PRD\nBody',
      surroundingText: 'Intro\n\nOutro',
    });
  });

  it('parses a story plan block', () => {
    const parsed = parseStoryPlan(`Plan ready
<story_plan>
{"summary":"Slice plan","proposed_stories":[{"ref":"story_1","name":"First","description":"Desc","story_type":"feature"}]}
</story_plan>`);

    expect(parsed?.plan.summary).toBe('Slice plan');
    expect(parsed?.plan.proposed_stories).toHaveLength(1);
    expect(parsed?.plan.proposed_stories[0]?.ref).toBe('story_1');
  });

  it('parses an approval request block', () => {
    const parsed = parseApprovalRequest(`Please review.
<approval_request phase="stories">
  <title>Story plan approval</title>
  <summary>Three vertical slices are ready.</summary>
</approval_request>`);

    expect(parsed).toEqual({
      phase: 'stories',
      title: 'Story plan approval',
      summary: 'Three vertical slices are ready.',
      surroundingText: 'Please review.',
    });
  });

  it('parses a self-closing approval request with attributes', () => {
    const parsed = parseApprovalRequest(`Please review.
<approval_request phase="prd" title="PRD approval" summary="The draft is ready for review" />`);

    expect(parsed).toEqual({
      phase: 'prd',
      title: 'PRD approval',
      summary: 'The draft is ready for review',
      surroundingText: 'Please review.',
    });
  });
});
