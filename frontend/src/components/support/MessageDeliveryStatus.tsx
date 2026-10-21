import { AlertCircleIcon, BubbleChatIcon, Clock01Icon, Loading03Icon, Mail01Icon, Tick01Icon, TickDouble01Icon } from '@/lib/icons';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import type { MessageDelivery } from './messageDelivery';

export function MessageDeliveryStatus({ delivery, onClick }: { delivery: MessageDelivery; onClick: () => void }) {
  const { channels, state } = delivery;
  const ReceiptIcon = state === 'read' || state === 'delivered' ? TickDouble01Icon
    : state === 'sent' ? Tick01Icon : state === 'queued' ? Clock01Icon
      : state === 'sending' ? Loading03Icon : null;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          data-slot="message-delivery-status"
          data-delivery-state={state}
          aria-label={`${channels.map(channel => channel.description).join('; ')}. View message info`}
          onClick={onClick}
          className="inline-flex min-h-6 shrink-0 items-center gap-1.5 rounded px-0.5 text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {channels.map(({ channel, state: channelState }) => {
            const ChannelIcon = channel === 'email' ? Mail01Icon : BubbleChatIcon;
            return (
              <span key={channel} data-channel={channel} data-failed={channelState === 'failed'} aria-hidden="true" className={`inline-flex items-center gap-0.5 ${channelState === 'failed' ? 'text-red-600 dark:text-red-400' : ''}`}>
                <ChannelIcon className="h-3 w-3" />
                {channelState === 'failed' && <AlertCircleIcon className="h-3 w-3" />}
              </span>
            );
          })}
          {ReceiptIcon && (
            <span data-slot="delivery-receipt" aria-hidden="true" className={state === 'read' ? 'text-blue-600 dark:text-blue-400' : ''}>
              <ReceiptIcon className={`h-3.5 w-3.5 ${state === 'sending' ? 'animate-spin motion-reduce:animate-none' : ''}`} />
            </span>
          )}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" className="flex-col items-start gap-0.5">
        {channels.map(channel => <span key={channel.channel} className="break-words">{channel.description}</span>)}
      </TooltipContent>
    </Tooltip>
  );
}
