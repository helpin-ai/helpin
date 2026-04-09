import { useState } from 'react';
import { ArchiveIcon, DragDropVerticalIcon, InboxIcon, PencilEdit01Icon, PlusSignIcon } from '@/lib/icons';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { TeamInboxDialog } from '@/components/support/TeamInboxDialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { ICON_MAP } from '@/components/ui/icon-picker';
import { useArchiveMailbox, useReorderMailboxes, useSupportMailboxes } from '@/hooks/queries/useSupport';
import type { SupportMailbox } from '@/lib/pmTypes';
import { queryKeys } from '@/lib/queryKeys';

function SortableMailboxItem({
  mailbox,
  onEdit,
  onArchive,
  isArchiving,
}: {
  mailbox: SupportMailbox;
  onEdit: (mailbox: SupportMailbox) => void;
  onArchive: (mailbox: SupportMailbox) => void;
  isArchiving: boolean;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: mailbox.id,
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  const MailboxIcon = ICON_MAP[mailbox.icon] ?? ICON_MAP.inbox;

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`flex items-center justify-between rounded-xl border bg-card px-4 py-3 ${isDragging ? 'opacity-50 shadow-lg' : ''}`}
    >
      <div className="flex min-w-0 items-center gap-3">
        <button
          type="button"
          className="flex h-5 w-5 shrink-0 cursor-grab items-center justify-center text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing"
          {...attributes}
          {...listeners}
        >
          <DragDropVerticalIcon className="h-4 w-4" />
        </button>
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
        <Button variant="ghost" size="icon" onClick={() => onEdit(mailbox)}>
          <PencilEdit01Icon className="h-4 w-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          disabled={!mailbox.active || isArchiving}
          onClick={() => onArchive(mailbox)}
        >
          <ArchiveIcon className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

export function TeamInboxesTab({ workspaceId }: { workspaceId: string }) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingMailbox, setEditingMailbox] = useState<SupportMailbox | null>(null);

  const queryClient = useQueryClient();
  const { data: mailboxes = [], isLoading } = useSupportMailboxes(workspaceId);
  const archiveMailbox = useArchiveMailbox(workspaceId);
  const reorderMailboxes = useReorderMailboxes(workspaceId);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const openCreate = () => {
    setEditingMailbox(null);
    setDialogOpen(true);
  };

  const openEdit = (mailbox: SupportMailbox) => {
    setEditingMailbox(mailbox);
    setDialogOpen(true);
  };

  const confirm = useConfirm();

  const handleArchive = async (mailbox: SupportMailbox) => {
    const ok = await confirm({
      title: `Archive ${mailbox.name}?`,
      description: 'This inbox will be archived. You can restore it later.',
      confirmText: 'Archive',
      variant: 'destructive',
    });
    if (!ok) return;
    archiveMailbox.mutate(mailbox.id, {
      onSuccess: () => toast.success('Team inbox archived'),
    });
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = mailboxes.findIndex((m) => m.id === active.id);
    const newIndex = mailboxes.findIndex((m) => m.id === over.id);
    if (oldIndex === -1 || newIndex === -1) return;

    const reordered = arrayMove(mailboxes, oldIndex, newIndex);

    // Optimistically update the cache so dnd-kit animates smoothly
    queryClient.setQueryData(
      queryKeys.support.mailboxes(workspaceId),
      reordered,
    );

    // Persist in background — invalidation in onSuccess will reconcile
    reorderMailboxes.mutate(reordered.map((m) => m.id));
  };

  return (
    <div className="space-y-4">
      {mailboxes.length > 0 && (
        <div className="flex justify-end">
          <Button className="gap-2" onClick={openCreate}>
            <PlusSignIcon className="h-4 w-4" />
            New Team Inbox
          </Button>
        </div>
      )}
      <Card>
        <CardContent className="space-y-3">
          {isLoading && <p className="text-sm text-muted-foreground">Loading inboxes...</p>}
          {!isLoading && mailboxes.length === 0 && (
            <div className="flex flex-col items-center justify-center rounded-xl border border-dashed py-12 text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                <InboxIcon className="h-6 w-6 text-muted-foreground" />
              </div>
              <h3 className="mt-4 text-sm font-medium">No team inboxes</h3>
              <p className="mt-1 max-w-sm text-sm text-muted-foreground">
                Create a team inbox to route conversations to a linked team or selected members. The shared inbox stays available to everyone.
              </p>
              <Button className="mt-4 gap-2" onClick={openCreate}>
                <PlusSignIcon className="h-4 w-4" />
                Create Team Inbox
              </Button>
            </div>
          )}
          {mailboxes.length > 0 && (
            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
              <SortableContext items={mailboxes.map((m) => m.id)} strategy={verticalListSortingStrategy}>
                <div className="space-y-3">
                  {mailboxes.map((mailbox) => (
                    <SortableMailboxItem
                      key={mailbox.id}
                      mailbox={mailbox}
                      onEdit={openEdit}
                      onArchive={handleArchive}
                      isArchiving={archiveMailbox.isPending}
                    />
                  ))}
                </div>
              </SortableContext>
            </DndContext>
          )}
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
