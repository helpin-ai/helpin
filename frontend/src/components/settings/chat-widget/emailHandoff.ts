// Same defaults as the server. Email handoffs go by business hours only; the
// greeting and sign-off are added around these messages when the email is sent.
export const DEFAULT_EMAIL_HANDOFF_MESSAGE = "Thanks for getting in touch. I've passed your message to our support team, and someone from the team will reply to this email as soon as they can.\n\nIf there's anything you'd like to add in the meantime, just reply here.";
export const DEFAULT_EMAIL_HANDOFF_MESSAGE_AFTER_HOURS = "Thanks for getting in touch. I've passed your message to our support team.\n\nThe team is away right now and back {next_open}. Someone will reply to this email once they're back.\n\nIf there's anything you'd like to add in the meantime, just reply here.";
