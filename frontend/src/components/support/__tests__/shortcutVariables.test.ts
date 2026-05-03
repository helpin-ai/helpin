import { describe, expect, it } from 'vitest';
import { resolveShortcutVariables } from '../shortcutVariables';

describe('resolveShortcutVariables', () => {
  it('resolves scoped variables and fallback values at insert time', () => {
    const result = resolveShortcutVariables(
      'Hi {{customer.first_name | fallback: "there"}},\n\nI am {{agent.first_name}}.',
      {
        customer: { fullName: '', email: 'customer@example.com' },
        agent: { fullName: 'Chris Lee', email: 'chris@example.com' },
      },
    );

    expect(result).toBe('Hi there,\n\nI am Chris.');
  });

  it('keeps legacy variable names and default syntax working', () => {
    const result = resolveShortcutVariables(
      'Hi {{first_name | default: "there"}},\n\nI am {{agent_first_name}} from {{workspace_name}}.',
      {
        customer: { fullName: '', email: 'customer@example.com' },
        agent: { fullName: 'Chris Lee', email: 'chris@example.com' },
        workspaceName: 'Helpin',
      },
    );

    expect(result).toBe('Hi there,\n\nI am Chris from Helpin.');
  });

  it('leaves unknown variables empty instead of blocking insertion', () => {
    expect(resolveShortcutVariables('Plan: {{plan_name}}', {})).toBe('Plan: ');
  });
});
