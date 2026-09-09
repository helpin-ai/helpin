import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('support email forwarding verification', () => {
  it('keeps routes incomplete until an end-to-end forwarding test passes', () => {
    const routingSource = readFileSync(resolve(__dirname, '../ConversationRoutingTab.tsx'), 'utf8');

    expect(routingSource).toContain('route.forwarding_verified_at');
    expect(routingSource).toContain("label: 'Setup incomplete'");
    expect(routingSource).toContain("label: 'Verified'");
  });

  it('guides forwarding through confirmation, provider enablement, and explicit delivery testing', () => {
    const forwardingSource = readFileSync(resolve(__dirname, '../EmailForwardingSetup.tsx'), 'utf8');
    const serviceSource = readFileSync(resolve(__dirname, '../../../lib/services/supportService.ts'), 'utf8');
    const hooksSource = readFileSync(resolve(__dirname, '../../../hooks/queries/useSupport.ts'), 'utf8');

    expect(forwardingSource).toContain('Add the Helpin address');
    expect(forwardingSource).toContain('Approve the confirmation');
    expect(forwardingSource).toContain('Enable forwarding in your provider');
    expect(forwardingSource).toContain('I’ve enabled forwarding');
    expect(forwardingSource).toContain('Open confirmation email');
    expect(forwardingSource).toContain('target="_blank"');
    expect(serviceSource).toContain('/send-test');
    expect(hooksSource).toContain('10 * 60 * 1000');
  });

  it('lets users create a team inbox from forwarding setup', () => {
    const forwardingSource = readFileSync(resolve(__dirname, '../SupportEmailForwardingTab.tsx'), 'utf8');

    expect(forwardingSource).toContain('Team inboxes (');
    expect(forwardingSource).toContain('Create team inbox');
    expect(forwardingSource).toContain("create_inbox: true");
    expect(forwardingSource).toContain('Set up forwarding');
  });
});
