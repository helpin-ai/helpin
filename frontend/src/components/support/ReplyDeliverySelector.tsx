import { Button } from "@/components/ui/button";
import { QuietDropdown } from "@/components/design-system/quiet-dropdown";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { ArrowDown01Icon, BubbleChatIcon, Mail01Icon, type IconComponent } from "@/lib/icons";
import type { SupportReplyDeliveryMode } from "@/lib/pmTypes";
import { supportChannel } from "@/lib/supportChannels";

function DeliveryIcons({ mode, ChatIcon }: { mode: SupportReplyDeliveryMode; ChatIcon: IconComponent }) {
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

function DeliveryOptionLabel({ mode, chatName, ChannelIcon, compact = false }: { mode: SupportReplyDeliveryMode; chatName: string; ChannelIcon: IconComponent; compact?: boolean }) {
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
      {mode !== "email_only" && (
        <span className="inline-flex items-center gap-1.5">
          <ChannelIcon aria-hidden="true" className="size-3.5" />
          {mode === "chat_only" && !compact ? `${chatName} only` : chatName}
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
  providers,
  unavailableProvider,
  onOpen,
}: {
  mode: SupportReplyDeliveryMode;
  onChange: (mode: SupportReplyDeliveryMode, provider?: string) => void;
  disabled: boolean;
  email: string;
  emailUnavailableReason?: string;
  chatUnavailableReason?: string;
  portal?: boolean;
  channel?: string;
  providers: string[];
  unavailableProvider: boolean;
  onOpen?: () => void;
}) {
  const choices: { mode: SupportReplyDeliveryMode; provider?: string }[] = [];
  const chatProviders = providers.length ? providers : unavailableProvider ? [] : [undefined];
  for (const provider of chatProviders) {
    choices.push({ mode: "chat_only", provider }, { mode: "chat_and_email", provider });
  }
  choices.push({ mode: "email_only" });
  const options = choices.map(choice => {
    const spec = supportChannel(choice.provider);
    const chatName = spec?.label ?? (portal ? "Portal" : "Chat");
    const ChannelIcon = spec?.inbox?.icon ?? BubbleChatIcon;
    const label = choice.mode === "chat_only" ? `${chatName} only` : choice.mode === "chat_and_email" ? `${chatName} + email` : "Email only";
    const reason = choice.mode === "chat_only" ? chatUnavailableReason : choice.mode === "email_only" ? emailUnavailableReason : emailUnavailableReason || chatUnavailableReason;
    return {
      ...choice, value: choice.provider ? `${choice.provider}-${choice.mode}` : choice.mode,
      label, disabled: !!reason, tooltip: reason, chatName, ChannelIcon,
      content: <DeliveryOptionLabel mode={choice.mode} chatName={chatName} ChannelIcon={ChannelIcon} />,
    };
  });
  const current = options.find(option => option.mode === mode && (mode === "email_only" || option.provider === channel));
  const description = current?.tooltip ?? (!current ? 'Choose a delivery channel' : mode === "chat_only"
    ? channel ? `Sent to the customer in ${current.chatName}.` : portal ? "Visible in the customer portal without an email notification." : "Only visible in the customer’s chat widget."
    : `${current.label} to ${email}.`);
  return (
    <QuietDropdown
      label="Sending options"
      searchMode="off"
      onOpenChange={open => { if (open) onOpen?.(); }}
      selected={current ? [current.value] : []}
      onSelect={value => {
        const choice = options.find(option => option.value === value);
        if (choice) onChange(choice.mode, choice.provider);
      }}
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
          aria-label={`Sending options: ${current?.label ?? "Choose channel"}`}
          className="h-7 gap-1 rounded-l-full rounded-r-none border-r border-primary-foreground/25 px-2 text-xs"
        >
          {!current ? "Choose channel" : current.provider
            ? <DeliveryOptionLabel mode={mode} chatName={current.chatName} ChannelIcon={current.ChannelIcon} compact />
            : <DeliveryIcons mode={mode} ChatIcon={current.ChannelIcon} />}
          <ArrowDown01Icon aria-hidden="true" className="size-3 opacity-70" />
        </Button>
      }
      groups={[{ id: "channels", label: "Sending options", options }]}
    />
  );
}
