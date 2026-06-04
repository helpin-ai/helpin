import { describe, expect, it } from 'vitest';
import { buildSupportSenderEmailPreview } from '../SupportEmailSendersTab';

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
