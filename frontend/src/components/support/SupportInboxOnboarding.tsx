import {
  Message01Icon,
  PlusSignIcon,
  Settings02Icon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';

interface SupportInboxOnboardingProps {
  onWidgetSettingsClick: () => void;
  onCreateConversationClick: () => void;
}

export function SupportInboxOnboarding({
  onWidgetSettingsClick,
  onCreateConversationClick,
}: SupportInboxOnboardingProps) {
  return (
    <div className="flex h-full min-h-0 flex-1 items-center justify-center overflow-auto bg-muted/20 p-6">
      <div className="w-full max-w-lg rounded-lg border bg-card p-6 text-center shadow-sm">
        <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/40">
          <Message01Icon className="h-5 w-5 text-muted-foreground" />
        </div>
        <h2 className="mt-5 text-xl font-semibold">Start support conversations</h2>
        <p className="mx-auto mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
          Install the widget for live chats, or create a test conversation to try replies and routing.
        </p>

        <div className="mt-5 flex flex-col gap-2 sm:flex-row sm:justify-center">
          <Button size="sm" onClick={onWidgetSettingsClick}>
            <Settings02Icon className="h-4 w-4" />
            Install widget
          </Button>
          <Button size="sm" variant="outline" onClick={onCreateConversationClick}>
            <PlusSignIcon className="h-4 w-4" />
            Create test conversation
          </Button>
        </div>
      </div>
    </div>
  );
}
