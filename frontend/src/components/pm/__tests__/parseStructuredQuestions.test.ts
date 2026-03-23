// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';

import { parseStructuredQuestions } from '../parseStructuredQuestions';

describe('parseStructuredQuestions', () => {
  it('parses the legacy nested <text> format', () => {
    const parsed = parseStructuredQuestions(`
      Before
      <questions>
        <question id="q1">
          <text>Who is the primary user?</text>
          <option value="admin">Admins</option>
          <option value="other" freetext="true">Other</option>
        </question>
      </questions>
      After
    `);

    expect(parsed?.questions).toEqual([
      {
        id: 'q1',
        text: 'Who is the primary user?',
        options: [
          { value: 'admin', label: 'Admins', freetext: undefined },
          { value: 'other', label: 'Other', freetext: true },
        ],
      },
    ]);
    expect(parsed?.surroundingText).toContain('Before');
    expect(parsed?.surroundingText).toContain('After');
  });

  it('parses the compact question text attribute format used by interactive runs', () => {
    const parsed = parseStructuredQuestions(`
      Now I have enough context.
      <questions>
        <question id="q1" text="What is the primary driver?">
          <option value="new">New deployments</option>
          <option value="other" freetext="true">Other (please specify)</option>
        </question>
      </questions>
    `);

    expect(parsed?.questions).toEqual([
      {
        id: 'q1',
        text: 'What is the primary driver?',
        options: [
          { value: 'new', label: 'New deployments', freetext: undefined },
          { value: 'other', label: 'Other (please specify)', freetext: true },
        ],
      },
    ]);
    expect(parsed?.surroundingText).toContain('Now I have enough context.');
  });

  it('parses fallback bullet-list questions when the model omits <option> tags', () => {
    const parsed = parseStructuredQuestions(`
      <questions>
        <question id="q1">
          What is the primary driver for adding NATS support alongside Kafka? Is it for:
          - Cost reduction
          - Performance requirements
          - Simpler operations/maintenance
          - Other (please specify)
        </question>
      </questions>
    `);

    expect(parsed?.questions).toEqual([
      {
        id: 'q1',
        text: 'What is the primary driver for adding NATS support alongside Kafka? Is it for:',
        options: [
          { value: 'cost-reduction', label: 'Cost reduction', freetext: undefined },
          { value: 'performance-requirements', label: 'Performance requirements', freetext: undefined },
          { value: 'simpler-operations-maintenance', label: 'Simpler operations/maintenance', freetext: undefined },
          { value: 'other', label: 'Other (please specify)', freetext: true },
        ],
      },
    ]);
  });

  it('parses sibling <question> and <options> blocks emitted by planner runs', () => {
    const parsed = parseStructuredQuestions(`
      <questions>
        <question id="q1">What is the primary motivation?</question>
        <options>
          <option value="cost">Cost reduction</option>
          <option value="other" freetext="true">Other (please specify)</option>
        </options>
        <question id="q2">What is the deployment strategy?</question>
        <options>
          <option value="dual_write">Dual-write</option>
          <option value="config_per_instance">Per instance</option>
        </options>
      </questions>
    `);

    expect(parsed?.questions).toEqual([
      {
        id: 'q1',
        text: 'What is the primary motivation?',
        options: [
          { value: 'cost', label: 'Cost reduction', freetext: undefined },
          { value: 'other', label: 'Other (please specify)', freetext: true },
        ],
      },
      {
        id: 'q2',
        text: 'What is the deployment strategy?',
        options: [
          { value: 'dual_write', label: 'Dual-write', freetext: undefined },
          { value: 'config_per_instance', label: 'Per instance', freetext: undefined },
        ],
      },
    ]);
  });

  it('handles orphan <option> as sibling of <question> (LLM format drift)', () => {
    const parsed = parseStructuredQuestions(`
      <questions>
        <question>
          Regarding session state for the Rust sessionization: Do you want to leverage the existing Redis infrastructure or go with an embedded solution?
        </question>
        <question>
          For the Kafka-to-NATs migration window: Would you rather commit to a calendar timeline or tie it to quality gates?
        </question>
        <option value="other" freetext="true">Other (please specify)</option>
      </questions>
    `);

    expect(parsed).not.toBeNull();
    expect(parsed?.questions).toHaveLength(2);
    expect(parsed?.questions[0]?.text).toContain('Rust sessionization');
    expect(parsed?.questions[0]?.options).toHaveLength(1);
    expect(parsed?.questions[0]?.options[0]?.freetext).toBe(true);
    expect(parsed?.questions[1]?.text).toContain('Kafka-to-NATs');
    expect(parsed?.questions[1]?.options).toHaveLength(1);
    expect(parsed?.questions[1]?.options[0]?.freetext).toBe(true);
  });
});
