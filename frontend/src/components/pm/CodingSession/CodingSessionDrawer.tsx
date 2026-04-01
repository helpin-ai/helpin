import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { CodingSessionSurface } from './CodingSessionSurface';

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
      <SheetContent side="right" className="w-full border-l p-0 sm:w-[78vw] sm:!max-w-[78vw]">
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
