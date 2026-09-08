import { describe, expect, it } from 'vitest';
import { blankPlaybook, createPlaybookIntentKey, definitionFingerprint, newMilestoneKey, publishIssues } from '../crmPlaybookPresentation';

describe('Playbook UI policy', () => {
  it('starts as policy only, with optional PM tasks disallowed', () => {
    expect(blankPlaybook().policy.pm_tasks).toBe('not_allowed');
    expect(blankPlaybook().policy.outbound_messages).toBe('approval_required');
    expect(publishIssues(blankPlaybook())).toEqual(['Describe the customer outcome.', 'Choose who handles escalations.', 'Add at least one milestone.']);
  });
  it('checks success criteria for every milestone', () => {
    const definition = blankPlaybook();
    definition.milestones = [{ key: 'intent', name: 'Intent confirmed', success_criteria: ' ' }];
    expect(publishIssues(definition)).toContain('Give every milestone a name and success criteria.');
  });
  it('compares semantic data rather than object property order', () => {
    expect(definitionFingerprint({ b: 2, a: { c: 1 } })).toBe(definitionFingerprint({ a: { c: 1 }, b: 2 }));
    expect(definitionFingerprint(['first', 'second'])).not.toBe(definitionFingerprint(['second', 'first']));
  });
  it('keeps idempotency keys on retries, but not changed intents or revisions', () => {
    const key = createPlaybookIntentKey();
    const first = key({ expected_revision: 3, operation: 'publish' });
    expect(key({ operation: 'publish', expected_revision: 3 })).toBe(first);
    expect(key({ operation: 'publish', expected_revision: 4 })).not.toBe(first);
  });
  it('creates unique durable milestone keys accepted by the API', () => {
    const first = newMilestoneKey();
    expect(first).toMatch(/^[a-z][a-z0-9_]{0,63}$/);
    expect(newMilestoneKey()).not.toBe(first);
  });
});
