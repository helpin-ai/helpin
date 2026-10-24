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

    const emailSetupSource = readFileSync(resolve(__dirname, '../TeamInboxEmailSetup.tsx'), 'utf8');

    expect(source).toContain('setStep(4);');
    expect(source).toContain('Your team inbox is ready');
    expect(source).toContain('<TeamInboxEmailSetup');
    expect(emailSetupSource).toContain('Send messages received at your existing email address to this Helpin inbox.');
    expect(emailSetupSource).toContain('Choose and verify the email address customers see when your team replies.');
    expect(emailSetupSource).toContain('Set up forwarding');
    expect(emailSetupSource).toContain('Add sender address');
  });

  it('labels the inbox AI replies toggle positively and links to AI assistant settings', () => {
    const source = readFileSync(resolve(__dirname, '../TeamInboxDialog.tsx'), 'utf8');

    expect(source).not.toContain('Turn off AI replies');
    expect(source).toContain('checked={!form.aiRepliesDisabled}');
    expect(source).toContain('/settings/support-ai-assistant');
  });
});
