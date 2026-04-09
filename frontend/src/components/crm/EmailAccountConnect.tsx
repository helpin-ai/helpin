import { useState } from 'react';
import {
  Alert01Icon,
  Clock02Icon,
  Loading01Icon,
  Mail01Icon,
  PlusSignIcon,
  Shield02Icon,
  Delete01Icon,
} from '@/lib/icons';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useDisconnectEmailAccount, useEmailAccounts, usePurgeEmailAccount } from '@/hooks/queries/useCRM';
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

export function EmailAccountConnect({ workspaceId, memberId, showAll = false }: EmailAccountConnectProps) {
  const filters = showAll ? undefined : { member_id: memberId };
  const { data: accounts = [] } = useEmailAccounts(workspaceId, filters);
  const disconnectAccount = useDisconnectEmailAccount(workspaceId);
  const purgeAccount = usePurgeEmailAccount(workspaceId);
  const access = useWorkspaceAccess(workspaceId);
  const { isAdmin } = usePermissions(access.data);

  const [selectedAccount, setSelectedAccount] = useState<CRMEmailAccount | null>(null);
  const [connecting, setConnecting] = useState<'gmail' | 'microsoft' | null>(null);

  const handleConnect = async (provider: 'gmail' | 'microsoft') => {
    if (provider === 'microsoft') {
      toast.info('Microsoft integration coming soon');
      return;
    }

    setConnecting(provider);
    try {
      const { data, error } = await crmEmailService.initiateOAuth(workspaceId, provider);
      if (error || !data) {
        toast.error(error || 'Failed to initiate OAuth');
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
      setSelectedAccount(null);
    } catch {
      toast.error('Failed to delete mailbox history');
    }
  };

  const isWorking = disconnectAccount.isPending || purgeAccount.isPending;

  return (
    <Card className="border-dashed border-border/70 bg-gradient-to-br from-background via-background to-muted/20 shadow-none">
      <CardHeader className="pb-4">
        <CardTitle className="text-base">Email Accounts</CardTitle>
        <CardDescription>Connect Gmail mailboxes, preserve history on disconnect, and reconnect without losing your sync checkpoint.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {accounts.length === 0 && (
          <div className="rounded-2xl border border-dashed border-border/70 bg-muted/20 px-5 py-8 text-center">
            <Mail01Icon className="mx-auto h-8 w-8 text-muted-foreground/45" />
            <p className="mt-3 text-sm font-medium">No mailboxes connected</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Connect Gmail to sync conversations, keep timeline history in CRM, and resume cleanly after reconnect.
            </p>
          </div>
        )}

        {accounts.map((account) => {
          const disconnectedAt = account.disconnected_at
            ? new Date(account.disconnected_at).toLocaleDateString()
            : null;

          return (
            <div
              key={account.id}
              className="flex flex-col gap-3 rounded-2xl border border-border/70 bg-card/80 p-4 shadow-[0_1px_0_0_rgba(255,255,255,0.4)_inset]"
            >
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="flex h-9 w-9 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
                      <Mail01Icon className="h-4 w-4" />
                    </div>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold">{account.email_address}</p>
                      <p className="text-xs text-muted-foreground">
                        {account.normalized_email_address ?? account.email_address}
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
                        History preserved
                      </Badge>
                    )}
                  </div>
                </div>

                <Button variant="ghost" size="sm" onClick={() => setSelectedAccount(account)}>
                  {account.status === 'disconnected' ? 'Manage reconnect' : 'Manage'}
                </Button>
              </div>

              <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                <p>
                  {account.status === 'disconnected'
                    ? `Sync is paused${disconnectedAt ? ` since ${disconnectedAt}` : ''}. Reconnect Gmail to resume from the preserved checkpoint.`
                    : 'Disconnect keeps synced emails in CRM. Only admins can permanently purge mailbox history.'}
                </p>
                <p className="whitespace-nowrap text-right">
                  {account.last_synced_at
                    ? `Last sync ${new Date(account.last_synced_at).toLocaleString()}`
                    : 'No sync yet'}
                </p>
              </div>
            </div>
          );
        })}

        <div className="flex flex-wrap gap-2 pt-1">
          <Button variant="outline" size="sm" onClick={() => handleConnect('gmail')} disabled={!!connecting}>
            {connecting === 'gmail' ? (
              <Loading01Icon className="mr-1.5 h-3 w-3 animate-spin" />
            ) : (
              <PlusSignIcon className="mr-1.5 h-3 w-3" />
            )}
            Connect Gmail
          </Button>
          <Button variant="outline" size="sm" onClick={() => handleConnect('microsoft')} disabled={!!connecting}>
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
                      <DialogTitle className="truncate text-left text-xl">{selectedAccount.email_address}</DialogTitle>
                      <DialogDescription className="mt-1 text-left">
                        Disconnect pauses sync and keeps CRM history intact. Purge is permanent and removes synced email and calendar artifacts for this mailbox.
                      </DialogDescription>
                    </div>
                  </div>
                </DialogHeader>

                <div className="space-y-4 px-6 py-5">
                  <div className="grid gap-3">
                    <button
                      type="button"
                      onClick={handleDisconnect}
                      disabled={isWorking}
                      className="flex w-full items-start gap-3 rounded-2xl border border-border/70 bg-card px-4 py-4 text-left transition-colors hover:border-primary/40 hover:bg-primary/[0.03] disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <div className="mt-0.5 flex h-9 w-9 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                        {disconnectAccount.isPending ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <PlusSignIcon className="h-4 w-4" />}
                      </div>
                      <div className="space-y-1">
                        <p className="text-sm font-semibold">Disconnect and keep history</p>
                        <p className="text-sm text-muted-foreground">
                          Stops sync, clears OAuth tokens, preserves emails already stored in CRM, and lets the next reconnect resume from checkpoint.
                        </p>
                      </div>
                    </button>

                    {isAdmin && selectedAccount.has_synced_data && (
                      <button
                        type="button"
                        onClick={handlePurge}
                        disabled={isWorking}
                        className="flex w-full items-start gap-3 rounded-2xl border border-rose-500/25 bg-rose-500/[0.04] px-4 py-4 text-left transition-colors hover:border-rose-500/40 hover:bg-rose-500/[0.07] disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        <div className="mt-0.5 flex h-9 w-9 items-center justify-center rounded-2xl bg-rose-500/12 text-rose-600 dark:text-rose-300">
                          {purgeAccount.isPending ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <Delete01Icon className="h-4 w-4" />}
                        </div>
                        <div className="space-y-1">
                          <div className="flex items-center gap-2">
                            <p className="text-sm font-semibold text-foreground">Disconnect and delete synced history</p>
                            <Badge variant="outline" className="border-rose-500/30 text-[10px] text-rose-700 dark:text-rose-300">
                              Admin only
                            </Badge>
                          </div>
                          <p className="text-sm text-muted-foreground">
                            Permanently removes synced email threads, messages, contact links, calendar events, and the mailbox checkpoint. CRM contacts and deals remain.
                          </p>
                        </div>
                      </button>
                    )}
                  </div>

                  {!isAdmin && (
                    <div className="flex items-start gap-2 rounded-2xl border border-border/70 bg-muted/25 px-4 py-3 text-sm text-muted-foreground">
                      <Shield02Icon className="mt-0.5 h-4 w-4 shrink-0" />
                      <p>Only workspace admins can permanently purge synced mailbox history.</p>
                    </div>
                  )}

                  {isAdmin && !selectedAccount.has_synced_data && (
                    <div className="flex items-start gap-2 rounded-2xl border border-amber-500/20 bg-amber-500/[0.05] px-4 py-3 text-sm text-amber-800 dark:text-amber-200">
                      <Alert01Icon className="mt-0.5 h-4 w-4 shrink-0" />
                      <p>This mailbox has no synced history yet. A normal disconnect is enough.</p>
                    </div>
                  )}
                </div>

                <DialogFooter className="border-t border-border/70 px-6 py-4 sm:justify-between">
                  <p className="text-xs text-muted-foreground">
                    {selectedAccount.status === 'disconnected'
                      ? 'This mailbox is already disconnected. Reconnecting Gmail will reuse the same mailbox record.'
                      : 'Reconnect later with Gmail to continue incremental sync on the same mailbox record.'}
                  </p>
                  <Button variant="outline" onClick={() => setSelectedAccount(null)} disabled={isWorking}>
                    Close
                  </Button>
                </DialogFooter>
              </>
            )}
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>
  );
}
