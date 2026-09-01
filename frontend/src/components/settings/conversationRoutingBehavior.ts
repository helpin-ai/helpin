export const AUTO_MOVE_ROUTING_TITLE = 'Automatically move conversations';

export const AUTO_MOVE_ROUTING_DESCRIPTION =
  'Move matched conversations to the selected inbox. Turn this off to show routing suggestions instead.';

type AutomatedRoutingState = {
  triage_enabled: boolean;
  triage_auto_move_enabled: boolean;
};

export function applyAutomatedRoutingEnabled<T extends AutomatedRoutingState>(
  draft: T,
  enabled: boolean,
): T {
  return {
    ...draft,
    triage_enabled: enabled,
    triage_auto_move_enabled: enabled ? true : draft.triage_auto_move_enabled,
  };
}
