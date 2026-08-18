import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('inbox sender fallback', () => {
  it('uses the workspace default sender for team inboxes without an override', () => {
    const source = readFileSync(resolve(__dirname, '../ConversationRoutingTab.tsx'), 'utf8');
    const fallbackBinding = 'emailSender={emailSenderByMailbox.get(mailbox.id) ?? workspaceDefaultSender}';

    expect(source.split(fallbackBinding)).toHaveLength(3);
  });
});
