import { useState } from 'react';
import { ExternalLink } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useChatSettings, useUpdateCustomerPortalSettings } from '@/hooks/queries/useSupport';
import { useSettingsAutosave } from '@/hooks/useSettingsAutosave';
import type { SupportInboxSettings } from '@/lib/pmTypes';
import { SettingsAutosaveGuard } from './SettingsAutosaveGuard';
import { SettingsSaveBar } from './SettingsSaveBar';
import { SettingsSaveStatus } from './SettingsSaveStatus';

type PortalSettingsDraft = Pick<
  SupportInboxSettings,
  'portal_enabled' | 'portal_intake_enabled' | 'portal_anonymous_intake_enabled'
>;

export function CustomerPortalSettings({ workspaceId, workspaceSlug, editable }: {
  workspaceId: string;
  workspaceSlug: string;
  editable: boolean;
}) {
  const query = useChatSettings(workspaceId);
  if (!query.data) {
    if (query.isError) {
      return (
        <div role="alert" className="space-y-3 text-sm">
          <p>Could not load customer portal settings.</p>
          <Button variant="outline" onClick={() => void query.refetch()}>Retry</Button>
        </div>
      );
    }
    return <Skeleton className="h-64 w-full max-w-3xl" />;
  }

  const settings = query.data.settings;
  const initialDraft: PortalSettingsDraft = {
    portal_enabled: settings.portal_enabled ?? false,
    portal_intake_enabled: settings.portal_intake_enabled ?? false,
    portal_anonymous_intake_enabled: settings.portal_anonymous_intake_enabled ?? false,
  };
  return <CustomerPortalSettingsEditor key={workspaceId} workspaceId={workspaceId} workspaceSlug={workspaceSlug} editable={editable} initialDraft={initialDraft} />;
}

function CustomerPortalSettingsEditor({ workspaceId, workspaceSlug, editable, initialDraft }: {
  workspaceId: string;
  workspaceSlug: string;
  editable: boolean;
  initialDraft: PortalSettingsDraft;
}) {
  const update = useUpdateCustomerPortalSettings(workspaceId);
  const [draft, setDraft] = useState<PortalSettingsDraft>(initialDraft);

  const autosave = useSettingsAutosave({
    scopeKey: workspaceId,
    enabled: editable,
    value: draft,
    savedValue: initialDraft,
    save: (value) => update.mutateAsync(value),
  });

  const portalURL = `${window.location.origin}/portal/${encodeURIComponent(workspaceSlug)}`;

  return (
    <div className="max-w-3xl space-y-8">
      {editable ? (
        <SettingsSaveBar>
          <SettingsAutosaveGuard isDirty={autosave.isDirty} error={autosave.error} onRetry={autosave.retry} />
          <span className="text-xs text-muted-foreground">Changes save automatically</span>
          <SettingsSaveStatus status={autosave.status} error={autosave.error} onRetry={autosave.retry} />
        </SettingsSaveBar>
      ) : (
        <p className="text-sm text-muted-foreground">A workspace support administrator can change these settings.</p>
      )}

      <section className="border-b border-border/70 pb-7" aria-labelledby="portal-address-heading">
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Customer access</p>
        <h2 id="portal-address-heading" className="mt-1 text-base font-semibold">Portal address</h2>
        <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-3">
          <span className="min-w-0 break-all text-sm text-foreground">{portalURL}</span>
          <a href={portalURL} target="_blank" rel="noopener noreferrer" className="inline-flex shrink-0 items-center gap-1.5 text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring">
            Open portal <ExternalLink className="size-3.5" aria-hidden="true" />
          </a>
        </div>
        <p className="mt-3 text-sm text-muted-foreground">Share this address with customers after enabling portal access.</p>
      </section>

      <section aria-labelledby="portal-access-heading">
        <h2 id="portal-access-heading" className="text-base font-semibold">Access and requests</h2>
        <p className="mt-1 text-sm text-muted-foreground">Choose who can access the portal and submit support requests.</p>
        <div className="mt-5 divide-y divide-border/70 border-y border-border/70">
          <div className="flex items-center justify-between gap-6 py-5">
            <div className="min-w-0">
              <Label htmlFor="portal-enabled" className="text-sm font-medium">Enable customer portal</Label>
              <p className="mt-1 text-sm text-muted-foreground">Let customers sign in and view their requests.</p>
            </div>
            <Switch id="portal-enabled" checked={draft.portal_enabled} disabled={!editable} onCheckedChange={(value) => setDraft((current) => ({ ...current, portal_enabled: value }))} />
          </div>
          <div className="flex items-center justify-between gap-6 py-5">
            <div className="min-w-0">
              <Label htmlFor="portal-intake-enabled" className="text-sm font-medium">Enable new requests</Label>
              <p className="mt-1 text-sm text-muted-foreground">Let signed-in customers create support requests.</p>
            </div>
            <Switch id="portal-intake-enabled" checked={draft.portal_intake_enabled} disabled={!editable} onCheckedChange={(value) => setDraft((current) => ({ ...current, portal_intake_enabled: value }))} />
          </div>
          <div className="flex items-center justify-between gap-6 py-5">
            <div className="min-w-0">
              <Label htmlFor="portal-anonymous-intake" className="text-sm font-medium">Allow requests without sign-in</Label>
              <p className="mt-1 text-sm text-muted-foreground">Let visitors submit a request with their email address.</p>
            </div>
            <Switch id="portal-anonymous-intake" checked={draft.portal_anonymous_intake_enabled} disabled={!editable} onCheckedChange={(value) => setDraft((current) => ({ ...current, portal_anonymous_intake_enabled: value }))} />
          </div>
        </div>
        {draft.portal_anonymous_intake_enabled && !draft.portal_intake_enabled && (
          <p className="mt-3 text-sm text-muted-foreground">Requests without sign-in become available when new requests are enabled.</p>
        )}
      </section>
    </div>
  );
}
