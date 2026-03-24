import { describe, expect, it } from 'vitest';

import {
  formatStructuredQuestionAnswers,
  hasAllStructuredQuestionAnswers,
  type StructuredQuestionAnswer,
} from '../StructuredQuestionCard';
import type { StructuredQuestion } from '@/lib/pmTypes';

describe('StructuredQuestionCard helpers', () => {
  it('formats selected answers into a visible structured reply', () => {
    const questions: StructuredQuestion[] = [
      {
        id: 'q1',
        type: 'single_select',
        text: 'Which segment should CRM prioritize?',
        options: [
          { value: 'enterprise', label: 'Enterprise' },
          { value: 'mid_market', label: 'Mid-market' },
        ],
      },
    ];
    const answers: Record<string, StructuredQuestionAnswer> = {
      q1: { value: 'enterprise', label: 'Enterprise' },
    };

    const formatted = formatStructuredQuestionAnswers(questions, answers);

    expect(formatted).toContain('Interactive question responses:');
    expect(formatted).toContain('- q1: Which segment should CRM prioritize? -> Enterprise');
    expect(formatted).toContain('"selected_value": "enterprise"');
    expect(formatted).toContain('"selected_label": "Enterprise"');
  });

  it('requires and formats freetext answers based on the option flag, not the literal value', () => {
    const questions: StructuredQuestion[] = [
      {
        id: 'q2',
        type: 'single_select',
        text: 'Which team should own this?',
        options: [
          { value: 'sales', label: 'Sales' },
          { value: 'custom_team', label: 'Other team', freetext: true },
        ],
      },
    ];

    const incompleteAnswers: Record<string, StructuredQuestionAnswer> = {
      q2: { value: 'custom_team', label: 'Other team' },
    };
    expect(hasAllStructuredQuestionAnswers(questions, incompleteAnswers)).toBe(false);

    const completeAnswers: Record<string, StructuredQuestionAnswer> = {
      q2: { value: 'custom_team', label: 'Other team', freetextValue: 'Revenue Operations' },
    };
    expect(hasAllStructuredQuestionAnswers(questions, completeAnswers)).toBe(true);

    const formatted = formatStructuredQuestionAnswers(questions, completeAnswers);
    expect(formatted).toContain('- q2: Which team should own this? -> Other team (Revenue Operations)');
    expect(formatted).toContain('"freetext": "Revenue Operations"');
  });
});
