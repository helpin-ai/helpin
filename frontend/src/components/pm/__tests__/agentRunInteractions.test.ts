import { describe, expect, it } from 'vitest';

import { parseMessageApprovalRequest, parseMessageStructuredQuestions } from '../agentRunInteractions';

describe('agentRunInteractions', () => {
  it('parses structured questions from human_input_request artifacts', () => {
    const parsed = parseMessageStructuredQuestions(
      {
        content: 'I need one last decision before I continue.',
        sequence_no: 7,
      } as never,
      [
        {
          artifact_type: 'human_input_request',
          inline_content: JSON.stringify({
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
          }),
          metadata: {
            assistant_message_sequence_no: 7,
          },
        },
      ] as never,
    );

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

  it('parses approval requests from human_approval_request artifacts', () => {
    const parsed = parseMessageApprovalRequest(
      {
        content: 'Review the proposed CRM action.',
        sequence_no: 4,
      } as never,
      [
        {
          artifact_type: 'human_approval_request',
          inline_content: JSON.stringify({
            phase: 'crm_review',
            title: 'Approve the stage change',
            summary: 'Move ACME to verbal commit.',
          }),
          metadata: {
            assistant_message_sequence_no: 4,
          },
        },
      ] as never,
    );

    expect(parsed).toEqual({
      phase: 'crm_review',
      title: 'Approve the stage change',
      summary: 'Move ACME to verbal commit.',
      surroundingText: 'Review the proposed CRM action.',
    });
  });

  it('returns null when no matching human_input_request artifact exists', () => {
    const parsed = parseMessageStructuredQuestions(
      {
        content: 'I need one last decision before I continue.',
        sequence_no: 7,
      } as never,
      [] as never,
    );

    expect(parsed).toBeNull();
  });

  it('returns null when no matching human_approval_request artifact exists', () => {
    const parsed = parseMessageApprovalRequest(
      {
        content: 'Review the proposed CRM action.',
        sequence_no: 9,
      } as never,
      [] as never,
    );

    expect(parsed).toBeNull();
  });
});
