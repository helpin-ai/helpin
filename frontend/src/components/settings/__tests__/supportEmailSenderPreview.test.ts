import { describe, expect, it } from 'vitest';
import { buildSupportSenderEmailPreview, deriveSendingDomainDefaults, groupSupportEmailSendersByDomain } from '../supportEmailSenderUtils';

describe('buildSupportSenderEmailPreview', () => {
  it('shows the agent-workspace From and Helpin conversation Reply-To contract', () => {
    const preview = buildSupportSenderEmailPreview({
      senderEmail: 'billing@acme.test',
      workspaceName: 'Acme',
      agentName: 'Arooj',
      replyDomain: 'replies.helpin.email',
    });

    expect(preview.from).toBe('Arooj - Acme <billing@acme.test>');
    expect(preview.replyTo).toBe('conv-{conversation_id}@replies.helpin.email');
  });

  it('falls back to Helpin defaults when optional labels are empty', () => {
    const preview = buildSupportSenderEmailPreview({
      senderEmail: 'support@helpin.email',
      workspaceName: '',
      agentName: '',
      replyDomain: '',
    });

    expect(preview.from).toBe('Agent - Workspace <support@helpin.email>');
    expect(preview.replyTo).toBe('conv-{conversation_id}@replies.helpin.email');
  });
});

describe('groupSupportEmailSendersByDomain', () => {
  it('groups sender addresses under their sending domains and keeps orphaned domains visible', () => {
    const groups = groupSupportEmailSendersByDomain(
      [
        { id: 'domain-1', domain: 'acme.com', status: 'verified', active: true },
        { id: 'domain-2', domain: 'example.com', status: 'pending_dns', active: false },
      ],
      [
        { id: 'sender-1', email: 'support@acme.com', domain: 'acme.com' },
        { id: 'sender-2', email: 'billing@acme.com', domain: 'acme.com' },
        { id: 'sender-3', email: 'help@missing.com', domain: 'missing.com' },
      ],
    );

    expect(groups.map((group) => group.domain)).toEqual(['acme.com', 'example.com', 'missing.com']);
    expect(groups[0].senders.map((sender) => sender.email)).toEqual(['billing@acme.com', 'support@acme.com']);
    expect(groups[1].senders).toEqual([]);
    expect(groups[2].senderDomain).toBeNull();
    expect(groups[2].senders.map((sender) => sender.email)).toEqual(['help@missing.com']);
  });
});

describe('deriveSendingDomainDefaults', () => {
  it('uses the workspace website domain for placeholders and prefill', () => {
    expect(deriveSendingDomainDefaults('Content Studio', 'https://www.contentstudio.io/pricing')).toEqual({
      domain: 'contentstudio.io',
      localPart: 'support',
      displayName: 'Content Studio Support',
      primaryExample: 'support@contentstudio.io',
      secondaryExample: 'billing@contentstudio.io',
    });
  });

  it('falls back to a workspace-name domain when no website exists', () => {
    expect(deriveSendingDomainDefaults('Test Docs', '')).toEqual({
      domain: 'testdocs.com',
      localPart: 'support',
      displayName: 'Test Docs Support',
      primaryExample: 'support@testdocs.com',
      secondaryExample: 'billing@testdocs.com',
    });
  });

  it('uses the parent domain when the workspace URL is an inbox subdomain', () => {
    expect(deriveSendingDomainDefaults('Content Studio', 'https://inbox.contentstudio.io')).toMatchObject({
      domain: 'contentstudio.io',
      localPart: 'support',
      primaryExample: 'support@contentstudio.io',
      secondaryExample: 'billing@contentstudio.io',
    });
  });
});
