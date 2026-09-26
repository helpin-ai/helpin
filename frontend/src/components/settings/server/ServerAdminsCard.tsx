import { useState, type FormEvent } from 'react';
import { toast } from 'sonner';
import { Card, CardContent } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { QuietIconAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { useGrantServerAdmin, useRevokeServerAdmin, useServerAdmins } from '@/hooks/queries/useInstance';
import type { ServerAdmin } from '@/lib/instanceTypes';
import { Delete01Icon, UserAdd01Icon } from '@/lib/icons';
import { useAuthStore } from '@/stores/authStore';

/** Settings → Signup & admins: the accounts that administer this server. */
export function ServerAdminsCard() {
  const admins = useServerAdmins();
  const grant = useGrantServerAdmin();
  const revoke = useRevokeServerAdmin();
  const currentUserId = useAuthStore((state) => state.user?.id);
  const [email, setEmail] = useState('');
  const [removing, setRemoving] = useState<ServerAdmin | null>(null);

  const add = async (event: FormEvent) => {
    event.preventDefault();
    const address = email.trim();
    if (!address) return;
    try {
      await grant.mutateAsync(address);
      setEmail('');
      toast.success(`${address} is now a server admin`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Couldn’t add the admin.');
    }
  };

  const list = admins.data ?? [];
  const lastAdmin = list.length <= 1;

  return (
    <Card className="rounded-lg border-border/70 py-0">
      <CardContent className="space-y-4 p-4">
        <div>
          <h2 className="text-sm font-semibold text-quiet-text-primary">Server admins</h2>
          <p className="mt-0.5 text-[12.5px] text-quiet-text-tertiary">They manage signup, server admins and application email for every workspace on this server.</p>
        </div>
        {admins.isLoading ? (
          <Skeleton className="h-10 w-80 max-w-full" />
        ) : admins.isError ? (
          <p className="text-sm text-quiet-text-tertiary">Couldn’t load the server admins.</p>
        ) : (
          <ul className="divide-y divide-quiet-divider-light border-y border-quiet-divider-light" aria-label="Server admins">
            {list.map((admin) => {
              const fromEnv = admin.source === 'env';
              const reason = fromEnv ? 'Set by HELPIN_ADMIN_EMAILS on the server' : lastAdmin ? 'Add another admin before removing this one' : null;
              return (
                <li key={admin.user_id} className="flex items-center gap-3 py-2.5">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-quiet-text-primary">
                      {admin.full_name || admin.email}
                      {admin.user_id === currentUserId && <span className="ml-1.5 text-[12px] font-normal text-quiet-text-tertiary">(you)</span>}
                    </p>
                    <p className="truncate text-[12px] text-quiet-text-tertiary">
                      {admin.email}{fromEnv && ' · Set by server configuration'}
                    </p>
                  </div>
                  <QuickTooltip label={reason ?? `Remove ${admin.email} as server admin`}>
                    <span>
                      <QuietIconAction
                        aria-label={`Remove ${admin.email} as server admin`}
                        disabled={Boolean(reason) || revoke.isPending}
                        onClick={() => setRemoving(admin)}
                      >
                        <Delete01Icon className="size-4" aria-hidden="true" />
                      </QuietIconAction>
                    </span>
                  </QuickTooltip>
                </li>
              );
            })}
          </ul>
        )}
        <form onSubmit={(event) => void add(event)} className="flex max-w-md items-end gap-3">
          <div className="min-w-0 flex-1 space-y-1.5">
            <Label htmlFor="server-admin-email">Add an admin</Label>
            <QuietUnderlineInput
              id="server-admin-email"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              placeholder="Email of an existing account"
              autoComplete="off"
            />
          </div>
          <QuietTextAction type="submit" disabled={!email.trim() || grant.isPending} className="pb-2">
            <UserAdd01Icon className="mr-1 size-4" aria-hidden="true" />
            {grant.isPending ? 'Adding…' : 'Add admin'}
          </QuietTextAction>
        </form>
      </CardContent>
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => { if (!open) setRemoving(null); }}
        title="Remove server admin?"
        description={removing?.user_id === currentUserId
          ? 'You will no longer be able to manage signup, server admins or application email.'
          : `${removing?.email ?? 'This account'} will no longer manage this server. Their workspace roles don’t change.`}
        confirmLabel="Remove admin"
        onConfirm={() => {
          const target = removing;
          setRemoving(null);
          if (!target) return;
          revoke.mutateAsync(target.user_id)
            .then(() => toast.success(`${target.email} is no longer a server admin`))
            .catch((error: unknown) => toast.error(error instanceof Error ? error.message : 'Couldn’t remove the admin.'));
        }}
      />
    </Card>
  );
}
