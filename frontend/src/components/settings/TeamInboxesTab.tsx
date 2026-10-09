import { useState, type ReactNode } from 'react';
import { ArchiveIcon, DragDropVerticalIcon, InboxIcon, PencilEdit01Icon, PlusSignIcon, UndoIcon } from '@/lib/icons';
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
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { StoredIcon } from '@/components/ui/icon-picker';
import { useArchiveMailbox, useReorderMailboxes, useSupportMailboxes, useUpdateMailbox } from '@/hooks/queries/useSupport';
import type { SupportMailbox } from '@/lib/pmTypes';
import { queryKeys } from '@/lib/queryKeys';

function SortableMailboxItem({
  mailbox,
  onEdit,
  onArchive,
  onRestore,
  isArchiving,
  isRestoring,
}: {
  mailbox: SupportMailbox;
  onEdit: (mailbox: SupportMailbox) => void;
  onArchive: (mailbox: SupportMailbox) => void;
  onRestore: (mailbox: SupportMailbox) => void;
  isArchiving: boolean;
  isRestoring: boolean;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: mailbox.id,
    disabled: !mailbox.active,
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`flex items-center justify-between rounded-xl border bg-card px-4 py-3 ${!mailbox.active ? 'opacity-60' : ''} ${isDragging ? 'opacity-50 shadow-lg' : ''}`}
    >
      <div className="flex min-w-0 items-center gap-3">
        {mailbox.active ? (
          <button
            type="button"
            className="flex h-5 w-5 shrink-0 cursor-grab items-center justify-center text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing"
            {...attributes}
            {...listeners}
          >
            <DragDropVerticalIcon className="h-4 w-4" />
          </button>
        ) : (
          <span className="h-5 w-5 shrink-0" />
        )}
        <StoredIcon
          name={mailbox.icon}
          className="h-4 w-4 text-muted-foreground"
          fallback={<InboxIcon className="h-4 w-4 text-muted-foreground" />}
        />
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <p className="truncate font-medium">{mailbox.name}</p>
            <Badge variant="secondary">{mailbox.assignment_mode === 'round_robin' ? 'Round robin' : mailbox.assignment_mode === 'specific_member' ? 'Specific member' : 'Manual'}</Badge>
            {!mailbox.active && <Badge variant="outline">Archived</Badge>}
          </div>
          <p className="text-xs text-muted-foreground">
            {mailbox.handle} • {mailbox.member_count ?? 0} people with access{mailbox.linked_team_name ? ` • linked to ${mailbox.linked_team_name}` : ''}
          </p>
        </div>
      </div>
      <div className="flex items-center gap-1">
        <IconButtonTooltip label="Edit">
          <Button variant="ghost" size="icon" onClick={() => onEdit(mailbox)} aria-label={`Edit ${mailbox.name}`}>
            <PencilEdit01Icon className="h-4 w-4" />
          </Button>
        </IconButtonTooltip>
        {mailbox.active ? (
          <IconButtonTooltip label="Archive">
            <Button
              variant="ghost"
              size="icon"
              disabled={isArchiving}
              onClick={() => onArchive(mailbox)}
              aria-label={`Archive ${mailbox.name}`}
            >
              <ArchiveIcon className="h-4 w-4 text-destructive" />
            </Button>
          </IconButtonTooltip>
        ) : (
          <IconButtonTooltip label="Restore">
            <Button
              variant="ghost"
              size="icon"
              disabled={isRestoring}
              onClick={() => onRestore(mailbox)}
              aria-label={`Restore ${mailbox.name}`}
            >
              <UndoIcon className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
            </Button>
          </IconButtonTooltip>
        )}
      </div>
    </div>
  );
}

function IconButtonTooltip({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}

export function TeamInboxesTab({ workspaceId }: { workspaceId: string }) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingMailbox, setEditingMailbox] = useState<SupportMailbox | null>(null);

  const queryClient = useQueryClient();
  const { data: mailboxes = [], isLoading } = useSupportMailboxes(workspaceId);
  const archiveMailbox = useArchiveMailbox(workspaceId);
  const updateMailbox = useUpdateMailbox(workspaceId);
  const reorderMailboxes = useReorderMailboxes(workspaceId);
  const activeMailboxes = mailboxes.filter((mailbox) => mailbox.active);
  const archivedMailboxes = mailboxes.filter((mailbox) => !mailbox.active);

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
      description: 'This hides the inbox from active lists. Conversations stay where they are.',
      confirmText: 'Archive',
      variant: 'destructive',
    });
    if (!ok) return;
    archiveMailbox.mutate(mailbox.id, {
      onSuccess: () => toast.success('Team inbox archived'),
    });
  };

  const handleRestore = (mailbox: SupportMailbox) => {
    updateMailbox.mutate({ mailboxId: mailbox.id, payload: { active: true } }, {
      onSuccess: () => toast.success('Team inbox restored'),
    });
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = activeMailboxes.findIndex((m) => m.id === active.id);
    const newIndex = activeMailboxes.findIndex((m) => m.id === over.id);
    if (oldIndex === -1 || newIndex === -1) return;

    const reordered = arrayMove(activeMailboxes, oldIndex, newIndex);

    // Optimistically update the cache so dnd-kit animates smoothly
    queryClient.setQueryData(
      queryKeys.support.mailboxes(workspaceId),
      [...reordered, ...archivedMailboxes],
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
              <SortableContext items={activeMailboxes.map((m) => m.id)} strategy={verticalListSortingStrategy}>
                <div className="space-y-3">
                  {activeMailboxes.map((mailbox) => (
                    <SortableMailboxItem
                      key={mailbox.id}
                      mailbox={mailbox}
                      onEdit={openEdit}
                      onArchive={handleArchive}
                      onRestore={handleRestore}
                      isArchiving={archiveMailbox.isPending}
                      isRestoring={updateMailbox.isPending}
                    />
                  ))}
                  {archivedMailboxes.map((mailbox) => (
                    <SortableMailboxItem
                      key={mailbox.id}
                      mailbox={mailbox}
                      onEdit={openEdit}
                      onArchive={handleArchive}
                      onRestore={handleRestore}
                      isArchiving={archiveMailbox.isPending}
                      isRestoring={updateMailbox.isPending}
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
