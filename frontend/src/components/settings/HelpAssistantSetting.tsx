import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useChatSettings, useUpdateCustomerPortalSettings } from '@/hooks/queries/useSupport';

/**
 * HelpAssistantSetting turns the Ask AI help assistant on or off. The help
 * center and the customer portal share this one setting, so it appears on
 * both settings pages; it saves as soon as it changes.
 */
export function HelpAssistantSetting({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const settings = useChatSettings(workspaceId);
  const update = useUpdateCustomerPortalSettings(workspaceId);
  const enabled = update.isPending ? update.variables?.help_assistant_enabled === true : settings.data?.settings.help_assistant_enabled === true;
  return (
    <div className="flex flex-col gap-4 rounded-lg border border-border/70 bg-card p-4 shadow-sm sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 space-y-1">
        <Label htmlFor={`help-assistant-${workspaceId}`}>Ask AI help assistant</Label>
        <p className="text-xs leading-relaxed text-muted-foreground">
          Visitors can ask questions in a conversation on the help center and in the customer portal. The Help assistant agent answers only from published articles, cites them, and offers support when the articles don&apos;t answer. Signed-in portal customers also get answers from spaces for signed-in customers. This setting is shared by the help center and the customer portal.
        </p>
      </div>
      <Switch
        id={`help-assistant-${workspaceId}`}
        checked={enabled}
        disabled={!editable || !settings.data || update.isPending}
        onCheckedChange={(checked) => update.mutate({ help_assistant_enabled: checked })}
        aria-label="Ask AI help assistant"
      />
    </div>
  );
}
