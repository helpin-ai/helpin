import { useNavigate } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import {
  QuietEmptyState,
  QuietPrimaryAction,
  QuietTextAction,
} from "@/components/design-system/quiet";
import { ArrowLeft02Icon } from "@/lib/icons";
import { BILLING_CHOOSE_PLAN_SEARCH } from "@/ee/lib/billingNavigation";

export const isContactLimitError = (error: unknown) =>
  error instanceof Error && error.message.includes("5,000 contacts");

export function ContactLimitNotice({
  workspaceSlug,
  onBack,
}: {
  workspaceSlug: string;
  onBack?: () => void;
}) {
  const navigate = useNavigate();
  const upgrade = () => {
    if (workspaceSlug)
      void navigate({
        to: "/w/$slug/settings/billing",
        params: { slug: workspaceSlug },
        search: BILLING_CHOOSE_PLAN_SEARCH,
      });
  };
  if (onBack)
    return (
      <QuietEmptyState
        title="Upgrade to view CRM contacts"
        description="The Starter plan includes up to 5,000 contacts. Support can keep capturing new contacts, but viewing CRM contacts above that limit requires the Growth plan."
        action={
          <div className="flex flex-wrap items-center gap-4">
            <QuietTextAction onClick={onBack}>
              <ArrowLeft02Icon className="h-3.5 w-3.5" />
              Back to contacts
            </QuietTextAction>
            <QuietPrimaryAction onClick={upgrade}>Upgrade</QuietPrimaryAction>
          </div>
        }
      />
    );
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
      <div>
        <h2 className="text-sm font-medium">Upgrade to view CRM contacts</h2>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          The Starter plan includes up to 5,000 contacts. Support can keep
          capturing new contacts, but CRM contact viewing requires the Growth
          plan once you exceed that limit.
        </p>
      </div>
      <Button size="sm" onClick={upgrade}>
        Upgrade
      </Button>
    </div>
  );
}
