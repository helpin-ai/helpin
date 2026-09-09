export const DEFAULT_DELAYED_TEAM_REPLY_MINUTES = 5;
export const DEFAULT_DELAYED_TEAM_REPLY_MESSAGE = "Our team hasn’t been able to reply yet. You don’t need to keep this chat open. We’ll email you when someone responds.";
export const DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL = "Our team hasn’t been able to reply yet. Leave your email and we’ll notify you when someone responds, so you don’t have to wait here.";

export function isValidDelayedTeamReplyMinutes(value: number): boolean {
  return Number.isInteger(value) && value >= 1 && value <= 1440;
}
