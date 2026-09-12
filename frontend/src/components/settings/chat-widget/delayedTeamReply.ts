export const DEFAULT_DELAYED_TEAM_REPLY_MINUTES = 5;
export const DEFAULT_DELAYED_TEAM_REPLY_MESSAGE = "Looks like our team needs a little more time. We’ll reply in this chat and email you if you miss it. Thanks for your patience.";
export const DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL = "Our team hasn’t been able to reply yet. Leave your email and we’ll notify you when someone responds, so you don’t have to wait here.";

export function isValidDelayedTeamReplyMinutes(value: number): boolean {
  return Number.isInteger(value) && value >= 1 && value <= 1440;
}
