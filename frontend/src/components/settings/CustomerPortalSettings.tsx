import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { ExternalLink } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useChatSettings, useCustomerPortalAccessSummary, useSupportAgents, useUpdateCustomerPortalSettings } from '@/hooks/queries/useSupport';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useSettingsAutosave } from '@/hooks/useSettingsAutosave';
import type { CustomerPortalAccessMode, CustomerPortalAccessSummary, SupportInboxSettings } from '@/lib/pmTypes';
import { serializeQueryFilterGroup } from '@/lib/queryBuilder';
import { SettingsAutosaveGuard } from './SettingsAutosaveGuard';
import { SettingsSaveBar } from './SettingsSaveBar';
import { SettingsSaveStatus } from './SettingsSaveStatus';

type PortalSettingsDraft = Pick<
  SupportInboxSettings,
  'portal_enabled' | 'portal_intake_enabled' | 'portal_anonymous_intake_enabled'
> & { portal_access_mode: CustomerPortalAccessMode; portal_ai_mode: 'off' | 'internal_note' | 'ai_first'; portal_ai_agent_id: string };

const accessModes: { value: CustomerPortalAccessMode; label: string; description: string }[] = [
  {
    value: 'approved_contacts',
    label: 'Approved contacts',
    description: 'Only CRM contacts a support admin has allowed can sign in.',
  },
  {
    value: 'any_verified_email',
    label: 'Any verified email',
    description: 'Anyone who confirms their email address can sign in, unless blocked.',
  },
];

const allowedContactsFilter = serializeQueryFilterGroup({
  logic: 'and',
  rules: [{ field: 'portal_access', operator: 'is', value: 'allowed' }],
});

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
    portal_access_mode: settings.portal_access_mode === 'any_verified_email' ? 'any_verified_email' : 'approved_contacts',
    portal_ai_mode: settings.portal_ai_mode === 'ai_first' || settings.portal_ai_mode === 'internal_note' ? settings.portal_ai_mode : 'off',
    portal_ai_agent_id: settings.portal_ai_agent_id ?? '',
  };
  return <CustomerPortalSettingsEditor key={workspaceId} workspaceId={workspaceId} workspaceSlug={workspaceSlug} editable={editable} initialDraft={initialDraft} sharedAgentId={settings.ai_agent_id} />;
}

function CustomerPortalSettingsEditor({ workspaceId, workspaceSlug, editable, initialDraft, sharedAgentId }: {
  workspaceId: string;
  workspaceSlug: string;
  editable: boolean;
  initialDraft: PortalSettingsDraft;
  sharedAgentId: string | null;
}) {
  const update = useUpdateCustomerPortalSettings(workspaceId);
  const summary = useCustomerPortalAccessSummary(workspaceId, editable);
  const { data: supportAgents = [] } = useSupportAgents(workspaceId);
  const [draft, setDraft] = useState<PortalSettingsDraft>(initialDraft);
	const effectiveAgentId = draft.portal_ai_agent_id || sharedAgentId;

  const autosave = useSettingsAutosave({
    scopeKey: workspaceId,
    enabled: editable,
    value: draft,
    savedValue: initialDraft,
    save: (value) => update.mutateAsync(value),
  });

  // The server knows where the portal is served (the help center, or the app).
  const portalURL = summary.data?.public_url || `${window.location.origin}/portal/${encodeURIComponent(workspaceSlug)}`;
  // Intake without sign-in is only offered when agents can reply by email.
  const deliveryUnavailable = summary.data?.anonymous_intake_delivery_available === false;

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

      <section className="border-b border-border/70 pb-7" aria-labelledby="portal-signin-heading">
        <h2 id="portal-signin-heading" className="text-base font-semibold">Who can sign in</h2>
        <fieldset className="mt-4 divide-y divide-border/70 border-y border-border/70" disabled={!editable}>
          <legend className="sr-only">Portal access mode</legend>
          {accessModes.map((mode) => (
            <label key={mode.value} htmlFor={`portal-access-${mode.value}`} className="flex cursor-pointer items-start gap-3 py-4">
              <input
                id={`portal-access-${mode.value}`}
                type="radio"
                name="portal-access-mode"
                value={mode.value}
                checked={draft.portal_access_mode === mode.value}
                onChange={() => setDraft((current) => ({ ...current, portal_access_mode: mode.value }))}
                className="mt-1 size-4 accent-primary"
              />
              <span className="min-w-0">
                <span className="block text-sm font-medium">{mode.label}</span>
                <span className="mt-1 block text-sm text-muted-foreground">{mode.description}</span>
              </span>
            </label>
          ))}
        </fieldset>
        <p className="mt-3 text-sm text-muted-foreground">Blocked contacts can never sign in. Submitting a request never grants portal access.</p>
        {editable && <PortalAccessOverview workspaceSlug={workspaceSlug} summary={summary.data} loading={summary.isLoading} failed={summary.isError} />}
      </section>

      <section aria-labelledby="portal-access-heading">
        <h2 id="portal-access-heading" className="text-base font-semibold">Access and requests</h2>
        <p className="mt-1 text-sm text-muted-foreground">Choose whether the portal is on and how customers submit support requests.</p>
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
              <p className="mt-1 text-sm text-muted-foreground">Let visitors submit a request with their email address. Only customers who can sign in may view it in the portal; others get replies by email.</p>
            </div>
            <Switch
              id="portal-anonymous-intake"
              checked={draft.portal_anonymous_intake_enabled}
              disabled={!editable || (deliveryUnavailable && !draft.portal_anonymous_intake_enabled)}
              onCheckedChange={(value) => setDraft((current) => ({ ...current, portal_anonymous_intake_enabled: value }))}
            />
          </div>
        </div>
        {deliveryUnavailable && (
          <p role="note" className="mt-3 text-sm text-muted-foreground">
            Requests without sign-in need outbound email: an application email sender for confirmations, plus Redis and the Postmark reply server so agents can reply. Configure both to turn this on.
          </p>
        )}
        {draft.portal_anonymous_intake_enabled && !draft.portal_intake_enabled && (
          <p className="mt-3 text-sm text-muted-foreground">Requests without sign-in become available when new requests are enabled.</p>
        )}
      </section>

      <section className="border-b border-border/70 pb-7" aria-labelledby="portal-ai-heading">
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Customer replies</p>
        <h2 id="portal-ai-heading" className="mt-1 text-base font-semibold">AI replies</h2>
        <p className="mt-1 text-sm text-muted-foreground">Choose how a support agent helps with new portal requests. Chat and email AI settings are separate.</p>
        <div className="mt-5 grid gap-5 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="portal-ai-mode">Response mode</Label>
            <Select value={draft.portal_ai_mode} onValueChange={(value) => setDraft((current) => ({ ...current, portal_ai_mode: value as PortalSettingsDraft['portal_ai_mode'] }))}>
              <SelectTrigger id="portal-ai-mode" aria-label="Portal AI response mode" disabled={!editable}><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="off">Off — team replies</SelectItem>
                <SelectItem value="internal_note" disabled={!effectiveAgentId}>Private suggestions</SelectItem>
                <SelectItem value="ai_first" disabled={!effectiveAgentId}>Reply to customers</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">Private suggestions are generated for unassigned requests and stay in the team inbox. Customer replies are sent only after server checks pass.</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="portal-ai-agent">Support agent</Label>
            <Select value={draft.portal_ai_agent_id || 'shared'} onValueChange={(value) => setDraft((current) => ({ ...current, portal_ai_agent_id: value === 'shared' ? '' : value }))}>
              <SelectTrigger id="portal-ai-agent" aria-label="Portal support agent" disabled={!editable}><SelectValue placeholder="Select a support agent" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="shared" disabled={!sharedAgentId}>Use chat support agent{sharedAgentId ? ` (${supportAgents.find((agent) => agent.id === sharedAgentId)?.name ?? 'selected agent'})` : ''}</SelectItem>
                {supportAgents.map((agent) => <SelectItem key={agent.id} value={agent.id}>{agent.name}</SelectItem>)}
              </SelectContent>
            </Select>
            {!effectiveAgentId && <p className="text-xs text-muted-foreground">Select a support agent before enabling AI replies.</p>}
          </div>
        </div>
      </section>
    </div>
  );
}

function PortalAccessOverview({ workspaceSlug, summary, loading, failed }: {
  workspaceSlug: string;
  summary?: CustomerPortalAccessSummary;
  loading: boolean;
  failed: boolean;
}) {
  if (loading) return <Skeleton className="mt-5 h-16 w-full" />;
  if (failed || !summary) {
    return <p role="alert" className="mt-5 text-sm text-muted-foreground">Portal access counts could not be loaded.</p>;
  }
  return (
    <div className="mt-5 space-y-4" aria-label="Portal access overview">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <p className="text-sm">
          <span className="font-medium">{summary.allowed_contacts}</span>{' '}
          {summary.allowed_contacts === 1 ? 'contact is' : 'contacts are'} allowed to sign in.
        </p>
        <Link
          to="/w/$slug/crm/contacts"
          params={{ slug: workspaceSlug }}
          search={{ filters: allowedContactsFilter }}
          className="text-sm font-medium text-primary underline-offset-4 hover:underline"
        >
          View allowed contacts
        </Link>
      </div>
      {summary.conflicts > 0 && (
        <div role="status" className="border-l-2 border-destructive/60 pl-3">
          <p className="text-sm font-medium">
            {summary.conflicts} {summary.conflicts === 1 ? 'email needs' : 'emails need'} attention
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            These emails have more than one allowed contact, or an allowed and a blocked contact, so nobody can sign in with them. Leave exactly one contact allowed, or decide whether the address should be blocked.
          </p>
          <ul className="mt-2 space-y-1">
            {summary.conflict_emails.map((email) => (
              <li key={email}>
                <Link
                  to="/w/$slug/crm/contacts"
                  params={{ slug: workspaceSlug }}
                  search={{ search: email }}
                  className="break-all text-sm text-primary underline-offset-4 hover:underline"
                >
                  {email}
                </Link>
              </li>
            ))}
          </ul>
          {summary.conflicts > summary.conflict_emails.length && (
            <p className="mt-1 text-xs text-muted-foreground">Showing the first {summary.conflict_emails.length}.</p>
          )}
        </div>
      )}
    </div>
  );
}
