import { Button } from "@/components/ui/button";
import { QuietDropdown } from "@/components/design-system/quiet-dropdown";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { ArrowDown01Icon, BubbleChatIcon, Mail01Icon } from "@/lib/icons";
import type { SupportReplyDeliveryMode } from "@/lib/pmTypes";
import { REPLY_DELIVERY_LABELS } from "./replyDelivery";

function DeliveryIcons({ mode }: { mode: SupportReplyDeliveryMode }) {
  return (
    <>
      {mode !== "email_only" && (
        <BubbleChatIcon aria-hidden="true" className="size-3.5" />
      )}
      {mode !== "chat_only" && (
        <Mail01Icon aria-hidden="true" className="size-3.5" />
      )}
    </>
  );
}

export function ReplyDeliverySelector({
  mode,
  onChange,
  disabled,
  email,
  emailUnavailableReason,
  chatUnavailableReason,
  automaticFallback,
}: {
  mode: SupportReplyDeliveryMode;
  onChange: (mode: SupportReplyDeliveryMode) => void;
  disabled: boolean;
  email: string;
  emailUnavailableReason?: string;
  chatUnavailableReason?: string;
  automaticFallback: boolean;
}) {
  const unavailableReason =
    mode === "chat_only"
      ? chatUnavailableReason
      : mode === "email_only"
        ? emailUnavailableReason
        : emailUnavailableReason || chatUnavailableReason;
  const description =
    unavailableReason ||
    (automaticFallback
      ? mode === "email_only"
        ? `Email to ${email}, using this inbox’s automatic delivery rules.`
        : `Chat now, with automatic email fallback to ${email} if needed.`
      : mode === "chat_only"
        ? "Only visible in the customer’s chat widget."
        : `${REPLY_DELIVERY_LABELS[mode]} to ${email}. Email is queued even if the customer is online.`);
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
      contentClassName="w-72"
      contentProps={{ side: "top", align: "end" }}
      triggerWrapper={(trigger) => (
        <QuickTooltip label={description}>{trigger}</QuickTooltip>
      )}
      trigger={
        <Button
          type="button"
          size="sm"
          disabled={disabled}
          aria-label={`Sending options: ${REPLY_DELIVERY_LABELS[mode]}`}
          className="h-7 gap-1 rounded-l-full rounded-r-none border-r border-primary-foreground/25 px-2 text-xs"
        >
          <DeliveryIcons mode={mode} />
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
            const detail =
              reason ||
              (value === "chat_only"
                ? "Only in the chat widget"
                : value === "email_only"
                  ? `Email to ${email}; hidden from chat`
                  : `Chat and email to ${email}`);
            return {
              value,
              label: REPLY_DELIVERY_LABELS[value],
              disabled: !!reason,
              leading: (
                <span className="flex gap-1">
                  <DeliveryIcons mode={value} />
                </span>
              ),
              content: (
                <span className="min-w-0">
                  <span className="block">{REPLY_DELIVERY_LABELS[value]}</span>
                  <span className="mt-0.5 block whitespace-normal break-words text-xs text-muted-foreground">
                    {detail}
                  </span>
                </span>
              ),
            };
          }),
        },
      ]}
    />
  );
}
