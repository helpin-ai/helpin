// Existing installations persist this original default. Treat it like an empty
// override so enabling/disabling AI updates the greeting without rewriting data.
const TEAM_WELCOME_MESSAGE = 'Hi there! How can we help you today?';
const AI_WELCOME_MESSAGE = 'Hi! I’m your AI assistant. I can help with most questions, and you can ask to speak with our team anytime.';

export function isDefaultWelcomeMessage(message?: string): boolean {
  return !message?.trim() || message === TEAM_WELCOME_MESSAGE;
}

/** Shared by the live widget and settings; custom wording always takes priority. */
export function resolveWelcomeMessage(message: string | undefined, aiFirst: boolean): string {
  if (!isDefaultWelcomeMessage(message)) return message!;
  return aiFirst ? AI_WELCOME_MESSAGE : TEAM_WELCOME_MESSAGE;
}
