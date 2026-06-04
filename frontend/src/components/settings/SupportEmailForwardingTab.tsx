import { type ReactNode, useCallback, useState } from 'react';
import { ArrowDown01Icon, ArrowRight01Icon, Copy01Icon, InboxIcon, MailAdd01Icon, Delete01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ICON_MAP } from '@/components/ui/icon-picker';
import {
  useCreateSupportEmailRoute,
  useDisableSupportEmailRoute,
  useChatSettings,
  useSupportEmailRoutes,
  useSupportMailboxes,
  useUpdateChatSettings,
} from '@/hooks/queries/useSupport';
import type { SupportEmailRoute, SupportMailbox } from '@/lib/pmTypes';

export function SupportEmailForwardingTab({ workspaceId }: { workspaceId: string }) {
  const [showHowItWorks, setShowHowItWorks] = useState(false);
  const { data: mailboxes = [], isLoading: mailboxesLoading } = useSupportMailboxes(workspaceId);
  const { data: routes = [], isLoading: routesLoading } = useSupportEmailRoutes(workspaceId);
  const createRoute = useCreateSupportEmailRoute(workspaceId);
  const disableRoute = useDisableSupportEmailRoute(workspaceId);

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
  const busy = createRoute.isPending || disableRoute.isPending;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <button
            type="button"
            onClick={() => setShowHowItWorks((v) => !v)}
            className="mt-2 flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            {showHowItWorks ? <ArrowDown01Icon className="h-3.5 w-3.5" /> : <ArrowRight01Icon className="h-3.5 w-3.5" />}
            How it works
          </button>
          {showHowItWorks && (
            <ol className="mt-3 space-y-2 text-sm text-muted-foreground">
              <li className="flex items-start gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">1</span>
                <span>Enable forwarding for Shared Inbox or a Team Inbox below.</span>
              </li>
              <li className="flex items-start gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">2</span>
                <span>Copy the generated Helpin address and set it as the forwarding target in Gmail, Zoho, Outlook, or any provider.</span>
              </li>
              <li className="flex items-start gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">3</span>
                <span>The status updates here once the first forwarded email arrives.</span>
              </li>
            </ol>
          )}
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
                  />
                </div>
              </div>

              <Separator />

              {/* Team Inboxes section */}
              <div>
                <div className="flex items-center justify-between pb-2">
                  <span className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                    Team Inboxes
                  </span>
                  {mailboxes.length > 0 && (
                    <span className="text-[11px] text-muted-foreground">
                      {mailboxes.length} inbox{mailboxes.length !== 1 ? 'es' : ''}
                    </span>
                  )}
                </div>

                {mailboxes.length === 0 ? (
                  <div className="rounded-lg border border-dashed py-6 text-center text-sm text-muted-foreground">
                    Create a Team Inbox to get private forwarding addresses like billing@ or vip@.
                  </div>
                ) : (
                  <div className="divide-y divide-border/50">
                    {mailboxes.map((mailbox) => (
                      <MailboxEmailRouteRow
                        key={mailbox.id}
                        mailbox={mailbox}
                        route={routeByMailboxId.get(mailbox.id) ?? null}
                        busy={busy}
                        onEnable={() => enableRoute(mailbox.id)}
                        onDisable={(route) => disableExistingRoute(route)}
                        onCopy={handleCopy}
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
}: {
  mailbox: SupportMailbox;
  route: SupportEmailRoute | null;
  busy: boolean;
  onEnable: () => void | Promise<void>;
  onDisable: (route: SupportEmailRoute) => void | Promise<void>;
  onCopy: (address: string) => void | Promise<void>;
}) {
  const MailboxIcon = ICON_MAP[mailbox.icon] ?? ICON_MAP.inbox;
  return (
    <EmailRouteRow
      title={mailbox.name}
      description={mailbox.linked_team_name ? `Linked to ${mailbox.linked_team_name}` : 'Team inbox'}
      icon={MailboxIcon ? <MailboxIcon className="h-4 w-4 text-muted-foreground" /> : <InboxIcon className="h-4 w-4 text-muted-foreground" />}
      route={route}
      busy={busy}
      onEnable={onEnable}
      onDisable={() => route ? onDisable(route) : Promise.resolve()}
      onCopy={onCopy}
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
}: {
  title: string;
  description: string;
  icon: ReactNode;
  route: SupportEmailRoute | null;
  busy: boolean;
  onEnable: () => void | Promise<void>;
  onDisable: () => void | Promise<void>;
  onCopy: (address: string) => void | Promise<void>;
}) {
  const lastInbound = route?.last_inbound_at ? new Date(route.last_inbound_at).toLocaleString() : null;

  return (
    <div className="py-3 px-1">
      <div className="flex items-center gap-3">
        {/* Status dot */}
        <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${route ? 'bg-emerald-500' : 'bg-slate-300 dark:bg-slate-600'}`} />

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
        {route && !lastInbound && (
          <span className="hidden shrink-0 items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400 sm:inline-flex">
            <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
            Awaiting first email
          </span>
        )}
        {route && lastInbound && (
          <span className="hidden shrink-0 text-xs text-muted-foreground lg:inline">
            Last: {lastInbound}
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
              Enable
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
