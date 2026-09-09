import { emailForwardingInboxHref } from './emailForwardingLinks';
import { ForwardingSetupTransition } from './ForwardingSetupTransition';
import { type ReactNode, useCallback, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Copy01Icon, InboxIcon, LinkSquare01Icon, MailAdd01Icon, Delete01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { StoredIcon } from '@/components/ui/icon-picker';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  useCreateSupportEmailRoute,
  useDisableSupportEmailRoute,
  useSendSupportEmailRouteTest,
  useChatSettings,
  useSupportEmailRoutes,
  useSupportMailboxes,
  useUpdateChatSettings,
} from '@/hooks/queries/useSupport';
import type { SupportEmailRoute, SupportMailbox } from '@/lib/pmTypes';

export function SupportEmailForwardingTab({ workspaceId }: { workspaceId: string }) {
  const navigate = useNavigate();
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  const { data: mailboxes = [], isLoading: mailboxesLoading } = useSupportMailboxes(workspaceId);
  const { data: routes = [], isLoading: routesLoading } = useSupportEmailRoutes(workspaceId);
  const activeMailboxes = mailboxes.filter((mailbox) => mailbox.active);
  const createRoute = useCreateSupportEmailRoute(workspaceId);
  const disableRoute = useDisableSupportEmailRoute(workspaceId);
  const sendRouteTest = useSendSupportEmailRouteTest(workspaceId);

  const sharedRoute = routes.find((route) => !route.mailbox_id) ?? null;
  const routeByMailboxId = new Map(routes.filter((route) => route.mailbox_id).map((route) => [route.mailbox_id as string, route]));

  const handleCopy = async (address: string) => {
    try {
      await navigator.clipboard.writeText(address);
      toast.success('Forwarding address copied');
    } catch {
      toast.error('Could not copy forwarding address');
    }
  };

  const enableRoute = async (mailboxId: string | null) => {
    await createRoute.mutateAsync({ mailbox_id: mailboxId, source_address: null });
    toast.success(mailboxId ? 'Mailbox forwarding enabled' : 'Shared inbox forwarding enabled');
  };

  const confirm = useConfirm();

  const disableExistingRoute = useCallback(async (route: SupportEmailRoute) => {
    const ok = await confirm({
      title: 'Disable forwarding?',
      description: 'Existing conversations will remain, but new forwarded emails will stop landing here.',
      confirmText: 'Disable',
      variant: 'destructive',
    });
    if (!ok) return;
    await disableRoute.mutateAsync(route.id);
    toast.success('Forwarding disabled');
  }, [confirm, disableRoute]);

  const isLoading = mailboxesLoading || routesLoading;
  const busy = createRoute.isPending || disableRoute.isPending || sendRouteTest.isPending;

  const handleSendTest = async (routeId: string, sourceAddress: string) => {
    await sendRouteTest.mutateAsync({ routeId, sourceAddress });
    toast.success('Forwarding test sent. Waiting for it to return to Helpin.');
  };

  const openCreateTeamInbox = () => {
    if (!workspaceSlug) return;
    void navigate({
      to: '/w/$slug/settings/inboxes-routing',
      params: { slug: workspaceSlug },
      search: { tab: 'inboxes', create_inbox: true },
    });
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <p className="text-sm font-medium text-foreground">How it works</p>
          <ol className="mt-3 space-y-2 text-sm text-muted-foreground">
            <li className="flex items-start gap-2.5">
              <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">1</span>
              <span>Add the Helpin address in your email provider.</span>
            </li>
            <li className="flex items-start gap-2.5">
              <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">2</span>
              <span>Approve the provider confirmation in Helpin.</span>
            </li>
            <li className="flex items-start gap-2.5">
              <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">3</span>
              <span>Enable forwarding in your provider and save the change.</span>
            </li>
            <li className="flex items-start gap-2.5">
              <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">4</span>
              <span>Let Helpin test delivery automatically.</span>
            </li>
          </ol>
        </CardHeader>

        <CardContent className="space-y-4">
          {isLoading && <p className="text-sm text-muted-foreground">Loading forwarding routes...</p>}

          {!isLoading && (
            <>
              {/* Shared Inbox section */}
              <div>
                <div className="pb-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Shared Inbox
                </div>
                <div className="divide-y divide-border/50">
                  <EmailRouteRow
                    title="Shared Inbox"
                    description="Workspace-wide — all forwarded email lands here."
                    icon={<InboxIcon className="h-4 w-4 text-muted-foreground" />}
                    route={sharedRoute}
                    busy={busy}
                    onEnable={() => enableRoute(null)}
                    onDisable={() => sharedRoute ? disableExistingRoute(sharedRoute) : Promise.resolve()}
                    onCopy={handleCopy}
                    onSendTest={handleSendTest}
                  />
                </div>
              </div>

              <Separator />

              {/* Team Inboxes section */}
              <div>
                <div className="flex items-center justify-between gap-3 pb-2">
                  <span className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                    Team inboxes ({activeMailboxes.length})
                  </span>
                  <Button size="sm" variant="outline" onClick={openCreateTeamInbox} disabled={!workspaceSlug}>
                    Create team inbox
                  </Button>
                </div>

                {activeMailboxes.length === 0 ? (
                  <div className="rounded-lg border border-dashed py-6 text-center text-sm text-muted-foreground">
                    Create a Team Inbox to get private forwarding addresses like billing@ or vip@.
                  </div>
                ) : (
                  <div className="divide-y divide-border/50">
                    {activeMailboxes.map((mailbox) => (
                      <MailboxEmailRouteRow
                        key={mailbox.id}
                        mailbox={mailbox}
                        route={routeByMailboxId.get(mailbox.id) ?? null}
                        busy={busy}
                        onEnable={() => enableRoute(mailbox.id)}
                        onDisable={(route) => disableExistingRoute(route)}
                        onCopy={handleCopy}
                        onSendTest={handleSendTest}
                      />
                    ))}
                  </div>
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <ManualForwardedEmailSettings workspaceId={workspaceId} />
    </div>
  );
}

function ManualForwardedEmailSettings({ workspaceId }: { workspaceId: string }) {
  const { data, isLoading } = useChatSettings(workspaceId);
  const updateSettings = useUpdateChatSettings(workspaceId);
  const settings = data?.settings;

  const enabled = settings?.forwarded_email_detection_enabled ?? true;
  const minConfidence = settings?.forwarded_email_min_confidence ?? 80;
  const busy = isLoading || updateSettings.isPending;

  const patchSettings = (patch: {
    forwarded_email_detection_enabled?: boolean;
    forwarded_email_detection_mode?: string;
    forwarded_email_min_confidence?: number;
  }) => {
    updateSettings.mutate(patch, {
      onSuccess: () => toast.success('Manual forwarded email detection updated'),
    });
  };

  return (
    <Card>
      <CardContent className="space-y-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <h3 className="text-sm font-medium">Manual forwarded email detection</h3>
            <p className="max-w-2xl text-sm text-muted-foreground">
              When a teammate forwards a customer email to a Helpin inbox, use the original customer as the sender so support can reply directly.
            </p>
          </div>
          <Switch
            checked={enabled}
            onCheckedChange={(checked) => patchSettings({
              forwarded_email_detection_enabled: checked,
              forwarded_email_detection_mode: 'high_confidence_any_sender',
            })}
            disabled={busy}
            aria-label="Toggle manual forwarded email detection"
          />
        </div>

        <div className="max-w-40 space-y-2">
          <Label htmlFor="manual-forwarded-email-confidence">Minimum confidence</Label>
          <Input
            id="manual-forwarded-email-confidence"
            type="number"
            min={50}
            max={100}
            step={5}
            value={minConfidence}
            disabled={busy || !enabled}
            onChange={(event) => {
              const next = Number(event.target.value);
              if (!Number.isFinite(next)) return;
              patchSettings({ forwarded_email_min_confidence: Math.max(50, Math.min(100, next)) });
            }}
          />
        </div>
      </CardContent>
    </Card>
  );
}

function MailboxEmailRouteRow({
  mailbox,
  route,
  busy,
  onEnable,
  onDisable,
  onCopy,
  onSendTest,
}: {
  mailbox: SupportMailbox;
  route: SupportEmailRoute | null;
  busy: boolean;
  onEnable: () => void | Promise<void>;
  onDisable: (route: SupportEmailRoute) => void | Promise<void>;
  onCopy: (address: string) => void | Promise<void>;
  onSendTest: (routeId: string, sourceAddress: string) => void | Promise<void>;
}) {
  return (
    <EmailRouteRow
      title={mailbox.name}
      description={mailbox.linked_team_name ? `Linked to ${mailbox.linked_team_name}` : 'Team inbox'}
      icon={
        <StoredIcon
          name={mailbox.icon}
          className="h-4 w-4 text-muted-foreground"
          fallback={<InboxIcon className="h-4 w-4 text-muted-foreground" />}
        />
      }
      route={route}
      busy={busy}
      onEnable={onEnable}
      onDisable={() => route ? onDisable(route) : Promise.resolve()}
      onCopy={onCopy}
      onSendTest={onSendTest}
    />
  );
}

function EmailRouteRow({
  title,
  description,
  icon,
  route,
  busy,
  onEnable,
  onDisable,
  onCopy,
  onSendTest,
}: {
  title: string;
  description: string;
  icon: ReactNode;
  route: SupportEmailRoute | null;
  busy: boolean;
  onEnable: () => void | Promise<void>;
  onDisable: () => void | Promise<void>;
  onCopy: (address: string) => void | Promise<void>;
  onSendTest: (routeId: string, sourceAddress: string) => void | Promise<void>;
}) {
  const lastInbound = route?.last_inbound_at ? new Date(route.last_inbound_at).toLocaleString() : null;
  const verifiedAt = route?.forwarding_verified_at ? new Date(route.forwarding_verified_at).toLocaleString() : null;
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  const [sourceAddress, setSourceAddress] = useState(route?.source_address ?? '');
  const confirmationHref = route?.confirmation_conversation_id
    ? emailForwardingInboxHref(workspaceSlug, route.mailbox_id, route.confirmation_conversation_id)
    : null;
  const inboxHref = emailForwardingInboxHref(workspaceSlug, route?.mailbox_id);

  return (
    <div className="py-3 px-1">
      <div className="flex items-center gap-3">
        {/* Status dot */}
        <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${verifiedAt ? 'bg-emerald-500' : route ? 'bg-amber-500' : 'bg-slate-300 dark:bg-slate-600'}`} />

        {/* Icon */}
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted">{icon}</div>

        {/* Title + description */}
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">{title}</p>
          <p className="truncate text-xs text-muted-foreground">{description}</p>
        </div>

        {/* Forwarding address (only when active, hidden on mobile) */}
        {route && (
          <code className="hidden shrink-0 rounded bg-muted px-2 py-0.5 text-[12px] text-muted-foreground sm:block">
            {route.inbound_address}
          </code>
        )}

        {/* Status label */}
        {route && !verifiedAt && (
          <span className="hidden shrink-0 items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400 sm:inline-flex">
            <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
            Setup incomplete
          </span>
        )}
        {route && verifiedAt && (
          <span className="hidden shrink-0 text-xs text-muted-foreground lg:inline">
            Verified: {verifiedAt}
          </span>
        )}

        {/* Actions */}
        <div className="flex shrink-0 items-center gap-1">
          {route ? (
            <>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => onCopy(route.inbound_address)} disabled={busy}>
                    <Copy01Icon className="h-3.5 w-3.5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Copy forwarding address</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive" onClick={() => onDisable()} disabled={busy}>
                    <Delete01Icon className="h-3.5 w-3.5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Disable forwarding</TooltipContent>
              </Tooltip>
            </>
          ) : (
            <Button size="sm" className="gap-1.5" onClick={() => onEnable()} disabled={busy}>
              <MailAdd01Icon className="h-3.5 w-3.5" />
              Enable forwarding
            </Button>
          )}
        </div>
      </div>
      {route && (
        <ForwardingSetupTransition key={route.id} verified={Boolean(verifiedAt)}>
        <div className="ml-11 mt-3 rounded-lg border border-amber-200 bg-amber-50/70 p-4 dark:border-amber-900/60 dark:bg-amber-950/20">
          <p className="text-sm font-medium text-amber-950 dark:text-amber-100">Complete forwarding setup</p>
          <div className="mt-3 space-y-2 text-sm">
            <ForwardingStep complete={Boolean(route.confirmation_received_at)} label="Add the Helpin address">
              <p>Copy <span className="font-medium text-foreground">{route.inbound_address}</span> and add it as a forwarding destination in the mailbox that receives customer email.</p>
              <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
                <SetupLink href="https://support.google.com/mail/answer/10957?hl=en">Gmail setup</SetupLink>
                <SetupLink href="https://support.microsoft.com/en-us/outlook/mail/turn-automatic-forwarding-on-or-off-in-outlook">Outlook setup</SetupLink>
              </div>
            </ForwardingStep>
            <ForwardingStep complete={false} label="Approve the confirmation">
              {confirmationHref ? (
                <>
                  <p>Confirmation email received. Open it in Helpin and approve the provider’s forwarding request.</p>
                  <div className="mt-2"><SetupLink href={confirmationHref}>Open confirmation email</SetupLink></div>
                </>
              ) : (
                <>
                  <p>Your provider may send a confirmation email to Helpin. We’ll detect it automatically when it arrives.</p>
                  {inboxHref && <div className="mt-2"><SetupLink href={inboxHref}>Open {route.mailbox_id ? 'Team Inbox' : 'Shared Inbox'}</SetupLink></div>}
                </>
              )}
            </ForwardingStep>
            <ForwardingStep complete={false} label="Enable forwarding in your provider">
              <p>Return to the source mailbox, select the Helpin address as the forwarding destination, enable forwarding, and save the change.</p>
            </ForwardingStep>
            <ForwardingStep complete={false} label="Test delivery">
              <p>Enter the source mailbox address, then confirm that forwarding is enabled. Helpin will send the test and verify this inbox when it returns.</p>
            </ForwardingStep>
          </div>
          <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:items-end">
            <div className="min-w-0 flex-1">
              <Label htmlFor={`forwarding-source-${route.id}`} className="text-xs">Email address forwarding into Helpin</Label>
              <Input
                id={`forwarding-source-${route.id}`}
                type="email"
                value={sourceAddress}
                onChange={(event) => setSourceAddress(event.target.value)}
                placeholder="support@company.com"
                className="mt-1"
                disabled={busy}
              />
            </div>
            <Button
              type="button"
              variant="outline"
              onClick={() => onSendTest(route.id, sourceAddress.trim())}
              disabled={busy || !sourceAddress.trim()}
            >
              I&apos;ve enabled forwarding
            </Button>
          </div>
          {route.verification_sent_at && (
            <p className="mt-2 text-xs text-amber-800 dark:text-amber-200">
              Test sent {new Date(route.verification_sent_at).toLocaleString()}. Waiting for it to return.
            </p>
          )}
          {route.forwarding_last_error && <p className="mt-2 text-xs text-destructive">{route.forwarding_last_error}</p>}
        </div>
        </ForwardingSetupTransition>
      )}
      {route && verifiedAt && lastInbound && (
        <p className="ml-11 mt-2 text-xs text-muted-foreground">Last email received: {lastInbound}</p>
      )}
    </div>
  );
}

function ForwardingStep({ complete, label, children }: { complete: boolean; label: string; children?: ReactNode }) {
  return (
    <div className="flex items-start gap-2">
      <span className={complete ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-300'}>
        {complete ? '✓' : '○'}
      </span>
      <div className="min-w-0">
        <p className={complete ? 'font-medium text-foreground' : 'font-medium text-muted-foreground'}>{label}</p>
        {children && <div className="mt-0.5 text-muted-foreground">{children}</div>}
      </div>
    </div>
  );
}

function SetupLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 font-medium text-primary hover:underline">
      {children}
      <LinkSquare01Icon className="h-3 w-3" />
    </a>
  );
}
