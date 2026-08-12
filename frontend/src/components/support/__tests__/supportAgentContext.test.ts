import { describe, expect, it } from 'vitest';
import { buildSupportConversationPageContext } from '../supportAgentContext';

describe('buildSupportConversationPageContext', () => {
  it('uses the conversation title and identifies the support conversation', () => {
    expect(buildSupportConversationPageContext({
      id: 'conv-1',
      title: 'Refund request',
      customer_name: 'Maya',
      customer_email: 'maya@example.com',
    })).toEqual({
      entity_type: 'support_conversation',
      entity_id: 'conv-1',
      display_title: 'Refund request',
    });
  });

  it('falls back through customer identity to the conversation id', () => {
    expect(buildSupportConversationPageContext({ id: 'conv-2', title: '', customer_name: ' Maya ', customer_email: 'maya@example.com' })?.display_title).toBe('Maya');
    expect(buildSupportConversationPageContext({ id: 'conv-3', customer_email: 'support@example.com' })?.display_title).toBe('support@example.com');
    expect(buildSupportConversationPageContext({ id: 'conv-4' })?.display_title).toBe('Conversation conv-4');
    expect(buildSupportConversationPageContext(null)).toBeNull();
  });
});
