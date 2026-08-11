import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('workspace onboarding priority copy', () => {
  it('explains the three-priority setup choice before users reach the limit', () => {
    const source = readFileSync(resolve(__dirname, '../Workspaces.tsx'), 'utf8');

    expect(source).toContain('What would you like to set up first?');
    expect(source).toContain('Choose up to 3 priorities to personalize your setup guide. You can add or change them anytime.');
    expect(source).toContain('{selectedUseCases.length} of 3 selected');
    expect(source).toContain('You’ve selected 3 priorities. Deselect one to choose another.');
  });
});
