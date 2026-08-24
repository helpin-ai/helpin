import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet';

interface DealEmailThreadPanelProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  dealId: string;
  threadId?: string;
}

export function DealEmailThreadPanel({
  open,
  onOpenChange,
  workspaceId,
  dealId,
  threadId,
}: DealEmailThreadPanelProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="h-dvh overflow-hidden p-0 data-[side=right]:w-screen data-[side=right]:!max-w-none lg:data-[side=right]:w-[68vw] lg:data-[side=right]:!max-w-[920px]"
        showCloseButton
      >
        <SheetTitle className="sr-only">Email conversation</SheetTitle>
        {threadId ? (
          <EmailTimeline
            workspaceId={workspaceId}
            dealId={dealId}
            selectedThreadId={threadId}
            showComposeAction={false}
            focusThreadOnly
          />
        ) : null}
      </SheetContent>
    </Sheet>
  );
}
