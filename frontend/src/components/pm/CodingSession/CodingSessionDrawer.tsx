import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { CodingSessionSurface } from './CodingSessionSurface';

function stopOutsideDismissPropagation(event: Event) {
  event.stopPropagation();
  const originalEvent = (event as Event & { detail?: { originalEvent?: Event } }).detail?.originalEvent;
  originalEvent?.stopPropagation();
}

export function CodingSessionDrawer({
  sessionId,
  open,
  onOpenChange,
  title = 'Agent Session',
  description = 'Interactive transcript, approvals, artifacts, and session details.',
}: {
  sessionId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title?: string;
  description?: string;
}) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="z-[70] w-full border-l p-0 data-[side=right]:w-[78vw] data-[side=right]:sm:max-w-[78vw]"
        overlayClassName="z-[70]"
        onOpenAutoFocus={(event) => event.preventDefault()}
        onPointerDownOutside={stopOutsideDismissPropagation}
        onInteractOutside={stopOutsideDismissPropagation}
      >
        <SheetHeader className="sr-only">
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>{description}</SheetDescription>
        </SheetHeader>
        {sessionId ? (
          <div className="h-full min-h-0 overflow-hidden">
            <CodingSessionSurface sessionId={sessionId} embedded showBackToRuns={false} />
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}
