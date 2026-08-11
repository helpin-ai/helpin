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

  it('shows setup evidence and an action to send the forwarding test', () => {
    const forwardingSource = readFileSync(resolve(__dirname, '../SupportEmailForwardingTab.tsx'), 'utf8');
    const serviceSource = readFileSync(resolve(__dirname, '../../../lib/services/supportService.ts'), 'utf8');
    const hooksSource = readFileSync(resolve(__dirname, '../../../hooks/queries/useSupport.ts'), 'utf8');

    expect(forwardingSource).toContain('Complete forwarding setup');
    expect(forwardingSource).toContain('Helpin forwarding address ready');
    expect(forwardingSource).toContain('Confirmation email received');
    expect(forwardingSource).toContain('Confirm and enable forwarding in your provider');
    expect(forwardingSource).toContain('Send test');
    expect(serviceSource).toContain('/send-test');
    expect(hooksSource).toContain('10 * 60 * 1000');
  });
});
