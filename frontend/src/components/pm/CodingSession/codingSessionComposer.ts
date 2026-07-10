import type { CodingSession, CodingSessionInteraction } from '@/lib/pmTypes';

export type CodingSessionComposerMode = 'answer' | 'continue' | 'waiting';

export type CodingSessionComposerState = {
  visible: boolean;
  enabled: boolean;
  mode: CodingSessionComposerMode;
  placeholder: string;
  disabledReason?: string;
};

const STRUCTURED_RESPONSE_INTERACTIONS = new Set([
  'approval_request',
  'review_checkpoint',
  'permissions_approval',
  'command_execution_approval',
  'file_change_approval',
]);

const HIDDEN_COMPOSER: CodingSessionComposerState = {
  visible: false,
  enabled: false,
  mode: 'waiting',
  placeholder: '',
};

export function resolveCodingSessionComposerState(
  session: CodingSession | null,
  activeInteraction?: CodingSessionInteraction | null,
  loading = false,
): CodingSessionComposerState {
  if (loading) {
    return {
      visible: true,
      enabled: false,
      mode: 'waiting',
      placeholder: 'Agent is starting...',
      disabledReason: 'The run is still loading.',
    };
  }

  if (!session) return HIDDEN_COMPOSER;

  if (
    activeInteraction?.status === 'pending'
    && STRUCTURED_RESPONSE_INTERACTIONS.has(activeInteraction.interaction_kind)
  ) {
    return HIDDEN_COMPOSER;
  }

  if (session.status === 'completed') return HIDDEN_COMPOSER;

  if (session.status === 'failed' || session.status === 'cancelled') {
    return {
      visible: true,
      enabled: true,
      mode: 'continue',
      placeholder: 'Add instructions to continue this run...',
    };
  }

  if (session.status === 'queued') {
    return {
      visible: true,
      enabled: false,
      mode: 'waiting',
      placeholder: 'Agent is starting...',
      disabledReason: 'The run has not started yet.',
    };
  }

  if (session.status === 'running') {
    return {
      visible: true,
      enabled: false,
      mode: 'waiting',
      placeholder: 'Agent is working. You can answer when it asks for input.',
      disabledReason: 'The agent has not asked for input.',
    };
  }

  if (session.status === 'paused') {
    if (session.pause_reason === 'human_input' || session.pause_reason === 'awaiting_user_message') {
      return {
        visible: true,
        enabled: true,
        mode: 'answer',
        placeholder: session.pause_reason === 'awaiting_user_message' ? 'Reply to continue this chat...' : 'Answer the agent...',
      };
    }

    if (session.pause_reason === 'authentication') {
      return {
        visible: true,
        enabled: false,
        mode: 'waiting',
        placeholder: 'Complete sign-in before responding',
        disabledReason: 'Authentication is required before the agent can continue.',
      };
    }

    return {
      visible: true,
      enabled: false,
      mode: 'waiting',
      placeholder: 'Use the review controls to respond',
      disabledReason: 'This run is waiting for a structured response.',
    };
  }

  return HIDDEN_COMPOSER;
}
