import { useState } from 'react';
import { ArrowDown, ArrowUp, Archive, Inbox, Pencil, Plus } from 'lucide-react';
import { toast } from 'sonner';
import { TeamInboxDialog } from '@/components/support/TeamInboxDialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ICON_MAP } from '@/components/ui/icon-picker';
import { useArchiveMailbox, useReorderMailboxes, useSupportMailboxes } from '@/hooks/queries/useSupport';
import type { SupportMailbox } from '@/lib/pmTypes';

export function TeamInboxesTab({ workspaceId }: { workspaceId: string }) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingMailbox, setEditingMailbox] = useState<SupportMailbox | null>(null);

  const { data: mailboxes = [], isLoading } = useSupportMailboxes(workspaceId);
  const archiveMailbox = useArchiveMailbox(workspaceId);
  const reorderMailboxes = useReorderMailboxes(workspaceId);

  const openCreate = () => {
    setEditingMailbox(null);
    setDialogOpen(true);
  };

  const openEdit = (mailbox: SupportMailbox) => {
    setEditingMailbox(mailbox);
    setDialogOpen(true);
  };

  const moveMailbox = async (mailboxId: string, direction: -1 | 1) => {
    const currentIndex = mailboxes.findIndex((mailbox) => mailbox.id === mailboxId);
    if (currentIndex < 0) return;
    const targetIndex = currentIndex + direction;
    if (targetIndex < 0 || targetIndex >= mailboxes.length) return;
    const reordered = [...mailboxes];
    const [entry] = reordered.splice(currentIndex, 1);
    reordered.splice(targetIndex, 0, entry);
    await reorderMailboxes.mutateAsync(reordered.map((mailbox) => mailbox.id));
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div>
            <CardTitle>Team Inboxes</CardTitle>
            {mailboxes.length > 0 && (
              <CardDescription>
                Private inboxes can grant access through a linked team and extra individual members. The shared inbox stays workspace-wide.
              </CardDescription>
            )}
          </div>
          {mailboxes.length > 0 && (
            <Button className="gap-2" onClick={openCreate}>
              <Plus className="h-4 w-4" />
              New Team Inbox
            </Button>
          )}
        </CardHeader>
        <CardContent className="space-y-3">
          {isLoading && <p className="text-sm text-muted-foreground">Loading inboxes...</p>}
          {!isLoading && mailboxes.length === 0 && (
            <div className="flex flex-col items-center justify-center rounded-xl border border-dashed py-12 text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                <Inbox className="h-6 w-6 text-muted-foreground" />
              </div>
              <h3 className="mt-4 text-sm font-medium">No team inboxes</h3>
              <p className="mt-1 max-w-sm text-sm text-muted-foreground">
                Create a team inbox to route conversations to a linked team or selected members. The shared inbox stays available to everyone.
              </p>
              <Button className="mt-4 gap-2" onClick={openCreate}>
                <Plus className="h-4 w-4" />
                Create Team Inbox
              </Button>
            </div>
          )}
          {mailboxes.map((mailbox, index) => {
            const MailboxIcon = ICON_MAP[mailbox.icon] ?? ICON_MAP.inbox;
            return (
              <div key={mailbox.id} className="flex items-center justify-between rounded-xl border bg-background px-4 py-3">
                <div className="flex min-w-0 items-center gap-3">
                  {MailboxIcon ? <MailboxIcon className="h-4 w-4 text-muted-foreground" /> : null}
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="truncate font-medium">{mailbox.name}</p>
                      <Badge variant="secondary">{mailbox.assignment_mode === 'round_robin' ? 'Round robin' : 'Manual'}</Badge>
                      {!mailbox.active && <Badge variant="outline">Archived</Badge>}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {mailbox.handle} • {mailbox.member_count ?? 0} people with access{mailbox.linked_team_name ? ` • linked to ${mailbox.linked_team_name}` : ''}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <Button variant="ghost" size="icon" disabled={index === 0 || reorderMailboxes.isPending} onClick={() => moveMailbox(mailbox.id, -1)}>
                    <ArrowUp className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" disabled={index === mailboxes.length - 1 || reorderMailboxes.isPending} onClick={() => moveMailbox(mailbox.id, 1)}>
                    <ArrowDown className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" onClick={() => openEdit(mailbox)}>
                    <Pencil className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    disabled={!mailbox.active || archiveMailbox.isPending}
                    onClick={() => {
                      if (window.confirm(`Archive ${mailbox.name}?`)) {
                        archiveMailbox.mutate(mailbox.id, {
                          onSuccess: () => toast.success('Team inbox archived'),
                        });
                      }
                    }}
                  >
                    <Archive className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            );
          })}
        </CardContent>
      </Card>

      <TeamInboxDialog
        workspaceId={workspaceId}
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open);
          if (!open) {
            setEditingMailbox(null);
          }
        }}
        mailbox={editingMailbox}
      />
    </div>
  );
}
