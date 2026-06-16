import { describe, expect, it } from 'vitest';

import { DEFAULT_ROUTING_SETTINGS, buildRoutingDraft } from '../ConversationRoutingTab';

describe('conversation routing defaults', () => {
  it('enables automated routing by default', () => {
    expect(DEFAULT_ROUTING_SETTINGS.triage_enabled).toBe(true);
    expect(buildRoutingDraft(null).triage_enabled).toBe(true);
  });
});
