import { describe, expect, it } from 'vitest';

import { parseMessageApprovalRequest, parseMessageStructuredQuestions } from '../agentRunInteractions';

describe('agentRunInteractions', () => {
  it('parses structured questions from request_human_input tool invocations', () => {
    const parsed = parseMessageStructuredQuestions({
      content: 'I need one last decision before I continue.',
      tool_invocations: [
        {
          tool_name: 'request_human_input',
          input: {
            questions: [
              {
                id: 'q1',
                type: 'single_select',
                text: 'Which segment should CRM prioritize?',
                options: [
                  { value: 'enterprise', label: 'Enterprise' },
                  { value: 'other', label: 'Other', freetext: true },
                ],
              },
            ],
          },
        },
      ],
    } as never);

    expect(parsed).toEqual({
      questions: [
        {
          id: 'q1',
          type: 'single_select',
          text: 'Which segment should CRM prioritize?',
          options: [
            { value: 'enterprise', label: 'Enterprise', freetext: undefined },
            { value: 'other', label: 'Other', freetext: true },
          ],
        },
      ],
      surroundingText: 'I need one last decision before I continue.',
    });
  });

  it('parses approval requests from request_human_approval tool invocations', () => {
    const parsed = parseMessageApprovalRequest({
      content: 'Review the proposed CRM action.',
      tool_invocations: [
        {
          tool_name: 'request_human_approval',
          input: {
            phase: 'crm_review',
            title: 'Approve the stage change',
            summary: 'Move ACME to verbal commit.',
          },
        },
      ],
    } as never);

    expect(parsed).toEqual({
      phase: 'crm_review',
      title: 'Approve the stage change',
      summary: 'Move ACME to verbal commit.',
      surroundingText: 'Review the proposed CRM action.',
    });
  });
});
