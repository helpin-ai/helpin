import { MailboxCapacityPanel } from "./outreach/MailboxCapacity";
import { CRMEmailSignatureSettings } from './CRMEmailSignature';
import { useEffect, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import {
  Alert01Icon,
  Clock02Icon,
  Loading01Icon,
  Mail01Icon,
  PlusSignIcon,
  Delete01Icon,
  RotateLeft01Icon,
} from '@/lib/icons';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { QuietPrimaryAction, QuietTextAction } from '@/components/design-system/quiet';
import { CRMEmailSettingsSection } from './CRMEmailSettingsSection';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  useDisconnectEmailAccount,
  useEmailAccountDiagnostics,
  useEmailAccounts,
  usePurgeEmailAccount,
  useSyncEmailAccount,
} from '@/hooks/queries/useCRM';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import type { CRMEmailAccount } from '@/lib/crmTypes';
import { crmEmailService } from '@/lib/services/crmService';
import { cn } from '@/lib/utils';

interface EmailAccountConnectProps {
  workspaceId: string;
  memberId: string;
  showAll?: boolean;
}

const statusStyles: Record<string, string> = {
  connected: 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
  disconnected: 'border-amber-500/20 bg-amber-500/10 text-amber-700 dark:text-amber-300',
  pending_oauth: 'border-sky-500/20 bg-sky-500/10 text-sky-700 dark:text-sky-300',
  error: 'border-rose-500/20 bg-rose-500/10 text-rose-700 dark:text-rose-300',
};

const statusLabels: Record<string, string> = {
  connected: 'Connected',
  disconnected: 'Disconnected',
  pending_oauth: 'Waiting for OAuth',
  error: 'Needs attention',
};

const activeSyncPhases = new Set(['backfill', 'incremental', 'recovery']);

function getSyncPhase(account: CRMEmailAccount): string {
  const phase = account.sync_state?.phase;
  return typeof phase === 'string' ? phase : '';
}

function formatRelativeTime(value?: string): string | null {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return formatDistanceToNow(date, { addSuffix: true });
}

function getSyncStatus(account: CRMEmailAccount) {
  const phase = getSyncPhase(account);
  const isSyncing = activeSyncPhases.has(phase);
  const lastSynced = formatRelativeTime(account.last_synced_at);

  if (account.status === 'disconnected') {
    return { label: 'Sync paused', detail: 'Reconnect Google to resume syncing.', isSyncing: false, tone: 'muted' as const };
  }
  if (account.status === 'error' || phase === 'error') {
    return { label: 'Sync needs attention', detail: 'Open sync details to review the error.', isSyncing: false, tone: 'error' as const };
  }
  if (isSyncing) {
    const labels: Record<string, string> = {
      backfill: 'Importing email history',
      incremental: 'Checking for new activity',
      recovery: 'Repairing email sync',
    };
    return { label: labels[phase] ?? 'Syncing', detail: 'Emails and calendar events will appear as they are imported.', isSyncing: true, tone: 'active' as const };
  }
  if (lastSynced) {
    return { label: `Synced ${lastSynced}`, detail: 'Email and calendar sync is up to date.', isSyncing: false, tone: 'success' as const };
  }
  return { label: 'Preparing first sync', detail: 'Helpin will begin importing email and calendar activity shortly.', isSyncing: false, tone: 'active' as const };
}

function getGoogleConnectionError(error: string | null): string {
  if (error?.toLowerCase().includes('oauth not configured')) {
    return 'Google connection is not configured for this Helpin server yet.';
  }
  return error || 'Failed to connect Google';
}

export function EmailAccountConnect({ workspaceId, memberId, showAll = false }: EmailAccountConnectProps) {
  const filters = showAll ? undefined : { member_id: memberId };
  const { data: accounts = [] } = useEmailAccounts(workspaceId, filters);
  const visibleAccounts = accounts.filter(
    (account) => account.status !== 'pending_oauth' || account.email_address !== 'pending@oauth.local',
  );
  const disconnectAccount = useDisconnectEmailAccount(workspaceId);
  const purgeAccount = usePurgeEmailAccount(workspaceId);
  const syncAccount = useSyncEmailAccount(workspaceId);
  const access = useWorkspaceAccess(workspaceId);
  const { isAdmin } = usePermissions(access.data);

  const [selectedAccount, setSelectedAccount] = useState<CRMEmailAccount | null>(null);
  const [connecting, setConnecting] = useState<'gmail' | 'microsoft' | null>(null);
  const [purgeConfirmOpen, setPurgeConfirmOpen] = useState(false);
  const diagnostics = useEmailAccountDiagnostics(workspaceId, selectedAccount?.id ?? '', !!selectedAccount);

  useEffect(() => {
    const currentURL = new URL(window.location.href);
    const oauthStatus = currentURL.searchParams.get('oauth');
    if (!oauthStatus) return;
    if (oauthStatus === 'success') toast.success('Google account connected. Your first import has started.');
    if (oauthStatus === 'cancelled') toast.info('Google connection was cancelled.');
    if (oauthStatus === 'error') toast.error('Google account could not be connected. Please try again.');
    currentURL.searchParams.delete('oauth');
    window.history.replaceState({}, '', `${currentURL.pathname}${currentURL.search}${currentURL.hash}`);
  }, []);

  const handleConnect = async (provider: 'gmail' | 'microsoft') => {
    if (provider === 'microsoft') {
      toast.info('Microsoft integration coming soon');
      return;
    }

    setConnecting(provider);
    try {
      const { data, error } = await crmEmailService.initiateOAuth(workspaceId, provider);
      if (error || !data) {
        toast.error(getGoogleConnectionError(error));
        return;
      }
      window.location.href = data.redirect_url;
    } catch {
      toast.error('Failed to connect Gmail');
    } finally {
      setConnecting(null);
    }
  };

  const handleDisconnect = async () => {
    if (!selectedAccount) return;
    try {
      await disconnectAccount.mutateAsync(selectedAccount.id);
      toast.success('Mailbox disconnected. Synced history was preserved.');
      setSelectedAccount(null);
    } catch {
      toast.error('Failed to disconnect mailbox');
    }
  };

  const handlePurge = async () => {
    if (!selectedAccount) return;
    try {
      await purgeAccount.mutateAsync(selectedAccount.id);
      toast.success('Mailbox and synced history deleted');
      setPurgeConfirmOpen(false);
      setSelectedAccount(null);
    } catch {
      toast.error('Failed to delete mailbox history');
    }
  };

  const handleSync = async (mode: 'incremental' | 'historical') => {
    if (!selectedAccount) return;
    try {
      await syncAccount.mutateAsync({ id: selectedAccount.id, mode });
      toast.success(mode === 'historical' ? 'History reimport queued' : 'Mailbox sync queued');
      await diagnostics.refetch();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to start mailbox sync');
    }
  };

  const isWorking = disconnectAccount.isPending || purgeAccount.isPending || syncAccount.isPending;

  const ConnectAction = visibleAccounts.length === 0 ? QuietPrimaryAction : QuietTextAction;

  return (
    <section className="border-b border-quiet-divider-strong pb-5">
      <h2 className="text-sm font-medium">Mailboxes</h2>
      <div className="mt-3 space-y-3">
        {visibleAccounts.length === 0 && (
          <div className="rounded-lg border bg-muted/20 px-5 py-7 text-center">
            <Mail01Icon className="mx-auto h-7 w-7 text-muted-foreground" />
            <p className="mt-3 text-sm font-medium">No Google account connected</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Connect once to bring email conversations and upcoming calendar meetings into CRM.
            </p>
          </div>
        )}

        {visibleAccounts.map((account) => {
          const syncStatus = getSyncStatus(account);

          return (
            <div
              key={account.id}
              className="flex flex-col gap-3 border-b border-quiet-divider-light py-4 last:border-b-0"
            >
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                      <Mail01Icon className="h-4 w-4" />
                    </div>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold">{account.email_address}</p>
                      <p className="text-xs text-muted-foreground">
                        Gmail and Google Calendar
                      </p>
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="outline" className="text-xs capitalize">
                      {account.provider}
                    </Badge>
                    <Badge
                      variant="outline"
                      className={cn('text-xs', statusStyles[account.status] ?? statusStyles.error)}
                    >
                      {statusLabels[account.status] ?? account.status}
                    </Badge>
                    {account.has_synced_data && (
                      <Badge variant="secondary" className="gap-1 text-xs">
                        <Clock02Icon className="h-3 w-3" />
                        {account.status === 'disconnected' ? 'History kept' : 'Data imported'}
                      </Badge>
                    )}
                  </div>
                </div>

                <QuietTextAction onClick={() => setSelectedAccount(account)}>
                  Manage
                </QuietTextAction>
              </div>

              {account.member_id === memberId && <><CRMEmailSignatureSettings workspaceId={workspaceId} account={account} /><CRMEmailSettingsSection title="Sending limits" optionId="email-sending-limits"><MailboxCapacityPanel workspaceId={workspaceId} accountId={account.id} /></CRMEmailSettingsSection></>}
              <div
                className={cn(
                  'flex items-start gap-2 rounded-lg border px-3 py-2.5',
                  syncStatus.tone === 'success' && 'border-emerald-500/20 bg-emerald-500/[0.05]',
                  syncStatus.tone === 'error' && 'border-rose-500/20 bg-rose-500/[0.05]',
                  syncStatus.tone === 'active' && 'border-sky-500/20 bg-sky-500/[0.05]',
                  syncStatus.tone === 'muted' && 'border-border bg-muted/20',
                )}
                title={account.last_synced_at ? `Last synced ${new Date(account.last_synced_at).toLocaleString()}` : undefined}
              >
                {syncStatus.isSyncing ? (
                  <Loading01Icon className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-sky-600" />
                ) : syncStatus.tone === 'error' ? (
                  <Alert01Icon className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />
                ) : (
                  <Clock02Icon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                )}
                <div className="min-w-0">
                  <p className="text-xs font-medium text-foreground">{syncStatus.label}</p>
                  <p className="mt-0.5 text-xs text-muted-foreground">{syncStatus.detail}</p>
                </div>
              </div>
            </div>
          );
        })}

        <div className={cn('flex flex-wrap gap-2 pt-1', visibleAccounts.length === 0 && 'justify-center')}>
          <ConnectAction onClick={() => handleConnect('gmail')} disabled={!!connecting}>
            {connecting === 'gmail' ? (
              <Loading01Icon className="mr-1.5 h-3 w-3 animate-spin" />
            ) : (
              <PlusSignIcon className="mr-1.5 h-3 w-3" />
            )}
            Connect Google
          </ConnectAction>
          <Button variant="ghost" size="sm" disabled title="Microsoft support is coming soon">
            <PlusSignIcon className="mr-1.5 h-3 w-3" />
            Microsoft
            <Badge variant="secondary" className="ml-1.5 text-[10px]">
              Soon
            </Badge>
          </Button>
        </div>

        <Dialog open={!!selectedAccount} onOpenChange={(open) => !open && setSelectedAccount(null)}>
          <DialogContent className="max-w-xl border-border/70 bg-background/98 p-0 sm:rounded-3xl">
            {selectedAccount && (
              <>
                <DialogHeader className="gap-3 border-b border-border/70 px-6 py-6 text-left">
                  <div className="flex items-start gap-3">
                    <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
                      <Mail01Icon className="h-5 w-5" />
                    </div>
                    <div className="min-w-0">
                      <DialogTitle className="text-left text-xl">Email &amp; calendar sync</DialogTitle>
                      <DialogDescription className="mt-1 text-left">
                        Activity and controls for <span className="font-medium text-foreground">{selectedAccount.email_address}</span>
                      </DialogDescription>
                    </div>
                  </div>
                </DialogHeader>

                <div className="space-y-4 px-6 py-5">
                  {diagnostics.isLoading ? (
                    <div className="flex items-center gap-2 rounded-2xl border border-border/70 bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
                      <Loading01Icon className="h-4 w-4 animate-spin" /> Loading mailbox health…
                    </div>
                  ) : diagnostics.data ? (
                    <div className="space-y-3 rounded-2xl border border-border/70 bg-muted/15 p-4">
                      <div className="flex flex-wrap items-start justify-between gap-3">
                        <div>
                          <p className="text-sm font-semibold">{getSyncStatus(selectedAccount).label}</p>
                          <p className="mt-1 text-xs text-muted-foreground">
                            {diagnostics.data.counts.messages.toLocaleString()} emails · {diagnostics.data.counts.calendar_events.toLocaleString()} calendar events
                          </p>
                        </div>
                        <Badge variant="outline" className={cn('text-xs capitalize', statusStyles[selectedAccount.status] ?? statusStyles.error)}>
                          {diagnostics.data.sync.status || statusLabels[selectedAccount.status]}
                        </Badge>
                      </div>
                      {diagnostics.data.sync.last_error && (
                        <div className="flex items-start gap-2 rounded-xl border border-rose-500/20 bg-rose-500/[0.05] px-3 py-2 text-sm text-rose-700 dark:text-rose-200">
                          <Alert01Icon className="mt-0.5 h-4 w-4 shrink-0" />
                          <div>
                            <p className="font-medium">Last sync failed</p>
                            <p className="mt-0.5 text-xs opacity-90">{diagnostics.data.sync.last_error.message}</p>
                          </div>
                        </div>
                      )}
                      {selectedAccount.is_active && (
                        <div className="border-t border-border/60 pt-3">
                          <p className="mb-2 text-xs text-muted-foreground">Helpin syncs automatically. Use these controls to check now or import older history again.</p>
                          <div className="flex flex-wrap gap-2">
                            <Button variant="outline" size="sm" onClick={() => handleSync('incremental')} disabled={isWorking}>
                              {syncAccount.isPending ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <RotateLeft01Icon className="mr-1.5 h-3.5 w-3.5" />}
                              Sync now
                            </Button>
                            <Button variant="ghost" size="sm" onClick={() => handleSync('historical')} disabled={isWorking}>
                              Reimport history
                            </Button>
                          </div>
                        </div>
                      )}
                    </div>
                  ) : null}

                  <div className="rounded-2xl border border-border/70 bg-card p-4">
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p className="text-sm font-semibold">Account connection</p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {selectedAccount.is_active
                            ? 'Disconnecting stops future sync. Existing CRM email and calendar activity stays available.'
                            : 'Reconnect Google to resume syncing from the saved checkpoint.'}
                        </p>
                      </div>
                      {selectedAccount.is_active ? (
                        <Button variant="outline" size="sm" onClick={handleDisconnect} disabled={isWorking}>
                          {disconnectAccount.isPending && <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                          Disconnect
                        </Button>
                      ) : (
                        <Button size="sm" onClick={() => handleConnect('gmail')} disabled={!!connecting}>
                          {connecting === 'gmail' ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />}
                          Reconnect Google
                        </Button>
                      )}
                    </div>

                    {isAdmin && selectedAccount.has_synced_data && (
                      <div className="mt-4 flex flex-col gap-3 border-t border-border/60 pt-4 sm:flex-row sm:items-center sm:justify-between">
                        <div>
                          <p className="text-sm font-medium">Delete synced data</p>
                          <p className="mt-1 text-xs text-muted-foreground">Permanently removes this mailbox's synced emails and calendar events.</p>
                        </div>
                        <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive" onClick={() => setPurgeConfirmOpen(true)} disabled={isWorking}>
                          <Delete01Icon className="mr-1.5 h-3.5 w-3.5" />
                          Delete data
                        </Button>
                      </div>
                    )}
                  </div>

                  {isAdmin && !selectedAccount.has_synced_data && (
                    <div className="flex items-start gap-2 rounded-2xl border border-amber-500/20 bg-amber-500/[0.05] px-4 py-3 text-sm text-amber-800 dark:text-amber-200">
                      <Alert01Icon className="mt-0.5 h-4 w-4 shrink-0" />
                      <p>This mailbox has no synced history yet. A normal disconnect is enough.</p>
                    </div>
                  )}
                </div>

                <DialogFooter className="border-t border-border/70 px-6 py-4">
                  <Button variant="outline" onClick={() => setSelectedAccount(null)} disabled={isWorking}>
                    Close
                  </Button>
                </DialogFooter>
              </>
            )}
          </DialogContent>
        </Dialog>
        <ConfirmDialog
          open={purgeConfirmOpen}
          onOpenChange={setPurgeConfirmOpen}
          title="Delete synced email and calendar data?"
          description={
            <>
              This permanently removes synced emails and calendar events for <strong>{selectedAccount?.email_address}</strong>. CRM contacts and deals will remain. This cannot be undone.
            </>
          }
          confirmLabel={purgeAccount.isPending ? 'Deleting…' : 'Delete synced data'}
          onConfirm={handlePurge}
        />
      </div>
    </section>
  );
}
