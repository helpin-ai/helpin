import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('TeamInboxDialog defaults', () => {
  it('starts new team inboxes with AI routing disabled', () => {
    const source = readFileSync(resolve(__dirname, '../TeamInboxDialog.tsx'), 'utf8');

    expect(source).toContain('triageEligible: false');
  });

  it('shows optional email setup after a new inbox is created', () => {
    const source = readFileSync(resolve(__dirname, '../TeamInboxDialog.tsx'), 'utf8');

    expect(source).toContain('setStep(4);');
    expect(source).toContain('Your team inbox is ready');
    expect(source).toContain('Send messages received at your existing email address to this Helpin inbox.');
    expect(source).toContain('Choose and verify the email address customers see when your team replies.');
    expect(source).toContain('Set up forwarding');
    expect(source).toContain('Add sender address');
  });
});
