import { Button } from "@/components/ui/button";
import { QuietDropdown } from "@/components/design-system/quiet-dropdown";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { ArrowDown01Icon, BubbleChatIcon, Mail01Icon, SlackIcon } from "@/lib/icons";
import type { SupportReplyDeliveryMode } from "@/lib/pmTypes";
import { REPLY_DELIVERY_LABELS } from "./replyDelivery";

function DeliveryIcons({ mode, slack }: { mode: SupportReplyDeliveryMode; slack: boolean }) {
  const ChatIcon = slack ? SlackIcon : BubbleChatIcon;
  return (
    <>
      {mode !== "email_only" && (
        <ChatIcon aria-hidden="true" className="size-3.5" />
      )}
      {mode !== "chat_only" && (
        <Mail01Icon aria-hidden="true" className="size-3.5" />
      )}
    </>
  );
}

function DeliveryOptionLabel({ mode, chatName }: { mode: SupportReplyDeliveryMode; chatName: string }) {
  const ChannelIcon = chatName === "Slack" ? SlackIcon : BubbleChatIcon;
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
      {mode !== "email_only" && (
        <span className="inline-flex items-center gap-1.5">
          <ChannelIcon aria-hidden="true" className="size-3.5" />
          {mode === "chat_only" ? `${chatName} only` : chatName}
        </span>
      )}
      {mode === "chat_and_email" && <span>+</span>}
      {mode !== "chat_only" && (
        <span className="inline-flex items-center gap-1.5">
          <Mail01Icon aria-hidden="true" className="size-3.5" />
          {mode === "email_only" ? "Email only" : "email"}
        </span>
      )}
    </span>
  );
}

export function ReplyDeliverySelector({
  mode,
  onChange,
  disabled,
  email,
  emailUnavailableReason,
  chatUnavailableReason,
  portal = false,
  channel,
}: {
  mode: SupportReplyDeliveryMode;
  onChange: (mode: SupportReplyDeliveryMode) => void;
  disabled: boolean;
  email: string;
  emailUnavailableReason?: string;
  chatUnavailableReason?: string;
  portal?: boolean;
  /** A messaging app (Telegram) that carries "chat" replies. */
  channel?: string;
}) {
  const chatName = channel ?? (portal ? "Portal" : "Chat");
  const labels: Record<SupportReplyDeliveryMode, string> = channel || portal ? {
    chat_only: `${chatName} only`, chat_and_email: `${chatName} + email`, email_only: "Email only",
  } : REPLY_DELIVERY_LABELS;
  const unavailableReason =
    mode === "chat_only"
      ? chatUnavailableReason
      : mode === "email_only"
        ? emailUnavailableReason
        : emailUnavailableReason || chatUnavailableReason;
  const description =
    unavailableReason ||
    (mode === "chat_only"
      ? channel ? `Sent to the customer in ${channel}.` : portal ? "Visible in the customer portal without an email notification." : "Only visible in the customer’s chat widget."
      : `${labels[mode]} to ${email}.`);
  const options: SupportReplyDeliveryMode[] = [
    "chat_only",
    "chat_and_email",
    "email_only",
  ];
  return (
    <QuietDropdown
      label="Sending options"
      searchMode="off"
      selected={[mode]}
      onSelect={(value) => onChange(value as SupportReplyDeliveryMode)}
      disabled={disabled}
      contentClassName="w-60"
      contentProps={{ side: "top", align: "end" }}
      triggerWrapper={(trigger) => (
        <QuickTooltip label={description}>{trigger}</QuickTooltip>
      )}
      trigger={
        <Button
          type="button"
          size="sm"
          disabled={disabled}
          aria-label={`Sending options: ${labels[mode]}`}
          className="h-7 gap-1 rounded-l-full rounded-r-none border-r border-primary-foreground/25 px-2 text-xs"
        >
          <DeliveryIcons mode={mode} slack={channel === "Slack"} />
          <ArrowDown01Icon aria-hidden="true" className="size-3 opacity-70" />
        </Button>
      }
      groups={[
        {
          id: "channels",
          label: "Sending options",
          options: options.map((value) => {
            const reason =
              value === "chat_only"
                ? chatUnavailableReason
                : value === "email_only"
                  ? emailUnavailableReason
                  : emailUnavailableReason || chatUnavailableReason;

            return {
              value,
              label: labels[value],
              disabled: !!reason,
              tooltip: reason,
              content: <DeliveryOptionLabel mode={value} chatName={chatName} />,
            };
          }),
        },
      ]}
    />
  );
}
