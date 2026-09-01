import { describe, expect, it } from 'vitest';

import {
  DEFAULT_ROUTING_SETTINGS,
  buildRoutingDraft,
} from '../ConversationRoutingTab';
import {
  AUTO_MOVE_ROUTING_DESCRIPTION,
  AUTO_MOVE_ROUTING_TITLE,
  applyAutomatedRoutingEnabled,
} from '../conversationRoutingBehavior';

describe('conversation routing defaults', () => {
  it('enables automated routing by default', () => {
    expect(DEFAULT_ROUTING_SETTINGS.triage_enabled).toBe(true);
    expect(buildRoutingDraft(null).triage_enabled).toBe(true);
  });

  it('enables auto-move whenever automated routing is turned on', () => {
    const disabledDraft = {
      ...DEFAULT_ROUTING_SETTINGS,
      triage_enabled: false,
      triage_auto_move_enabled: false,
    };

    expect(applyAutomatedRoutingEnabled(disabledDraft, true)).toMatchObject({
      triage_enabled: true,
      triage_auto_move_enabled: true,
    });
  });

  it('describes auto-move with positive switch language', () => {
    expect(AUTO_MOVE_ROUTING_TITLE).toBe('Automatically move conversations');
    expect(AUTO_MOVE_ROUTING_DESCRIPTION).toBe(
      'Move matched conversations to the selected inbox. Turn this off to show routing suggestions instead.',
    );
  });
});
