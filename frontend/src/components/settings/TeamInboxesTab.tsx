import { useEffect, useMemo, useState } from 'react';
import { ArrowDown, ArrowUp, ArrowRight, Archive, Check, ChevronLeft, HelpCircle, Inbox, Pencil, Plus, Search, Users } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Checkbox } from '@/components/ui/checkbox';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Badge } from '@/components/ui/badge';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { IconPicker, ICON_MAP } from '@/components/ui/icon-picker';
import { getInitials } from '@/lib/utils';
import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import { useArchiveMailbox, useCreateMailbox, useMailboxMembers, useReorderMailboxes, useSupportMailboxes, useUpdateMailbox } from '@/hooks/queries/useSupport';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { CreateSupportMailboxRequest, SupportMailbox, UpdateSupportMailboxRequest } from '@/lib/pmTypes';

type MailboxFormState = {
  name: string;
  handle: string;
  icon: string;
  description: string;
  linkedTeamId: string;
  assignmentMode: 'manual' | 'round_robin';
  workspaceMemberIds: string[];
};

const DEFAULT_FORM: MailboxFormState = {
  name: '',
  handle: '',
  icon: 'inbox',
  description: '',
  linkedTeamId: 'none',
  assignmentMode: 'manual',
  workspaceMemberIds: [],
};

function normalizeHandle(value: string) {
  return value.trim().toLowerCase().replace(/\s+/g, '-');
}

function buildFormState(mailbox?: SupportMailbox | null): MailboxFormState {
  if (!mailbox) return DEFAULT_FORM;
  return {
    name: mailbox.name,
    handle: mailbox.handle,
    icon: mailbox.icon || 'inbox',
    description: mailbox.description ?? '',
    linkedTeamId: mailbox.linked_team_id ?? 'none',
    assignmentMode: mailbox.assignment_mode,
    workspaceMemberIds: [],
  };
}

function FieldLabel({ htmlFor, children, tip }: { htmlFor?: string; children: React.ReactNode; tip: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <Label htmlFor={htmlFor}>{children}</Label>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="h-3.5 w-3.5 text-muted-foreground/60 cursor-help" />
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-56">
          {tip}
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

export function TeamInboxesTab({ workspaceId }: { workspaceId: string }) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [step, setStep] = useState<1 | 2>(1);
  const [editingMailbox, setEditingMailbox] = useState<SupportMailbox | null>(null);
  const [form, setForm] = useState<MailboxFormState>(DEFAULT_FORM);
  const [memberSearch, setMemberSearch] = useState('');

  const { data: mailboxes = [], isLoading } = useSupportMailboxes(workspaceId);
  const { data: members = [] } = useAssignableMembers(workspaceId);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { data: mailboxMembers = [] } = useMailboxMembers(workspaceId, editingMailbox?.id);
  const createMailbox = useCreateMailbox(workspaceId);
  const updateMailbox = useUpdateMailbox(workspaceId);
  const archiveMailbox = useArchiveMailbox(workspaceId);
  const reorderMailboxes = useReorderMailboxes(workspaceId);

  useEffect(() => {
    if (!dialogOpen) {
      setEditingMailbox(null);
      setForm(DEFAULT_FORM);
      setStep(1);
      setMemberSearch('');
    }
  }, [dialogOpen]);

  useEffect(() => {
    if (!editingMailbox) return;
    setForm((current) => ({
      ...current,
      workspaceMemberIds: mailboxMembers.map((member) => member.workspace_member_id),
    }));
  }, [editingMailbox, mailboxMembers]);

  const activeMembers = useMemo(
    () => members.filter((member) => member.status === 'active'),
    [members],
  );

  const filteredMembers = useMemo(() => {
    if (!memberSearch) return activeMembers;
    const q = memberSearch.toLowerCase();
    return activeMembers.filter(
      (m) =>
        m.display_name?.toLowerCase().includes(q) ||
        m.email?.toLowerCase().includes(q),
    );
  }, [activeMembers, memberSearch]);

  const canProceedToStep2 = form.name.trim().length > 0 && form.handle.trim().length > 0;

  const goToStep2 = () => {
    if (!form.name.trim()) { toast.error('Inbox name is required'); return; }
    if (!form.handle.trim()) { toast.error('Inbox handle is required'); return; }
    setStep(2);
  };

  const openCreate = () => {
    setEditingMailbox(null);
    setForm(DEFAULT_FORM);
    setDialogOpen(true);
  };

  const openEdit = (mailbox: SupportMailbox) => {
    setEditingMailbox(mailbox);
    setForm(buildFormState(mailbox));
    setDialogOpen(true);
  };

  const submit = async () => {
    if (!form.name.trim()) {
      toast.error('Inbox name is required');
      return;
    }
    if (!form.handle.trim()) {
      toast.error('Inbox handle is required');
      return;
    }

    if (editingMailbox) {
      const payload: UpdateSupportMailboxRequest = {
        name: form.name.trim(),
        handle: normalizeHandle(form.handle),
        icon: form.icon,
        description: form.description.trim() || null,
        linked_team_id: form.linkedTeamId === 'none' ? null : form.linkedTeamId,
        assignment_mode: form.assignmentMode,
        workspace_member_ids: form.workspaceMemberIds,
      };
      await updateMailbox.mutateAsync({ mailboxId: editingMailbox.id, payload });
      toast.success('Team inbox updated');
    } else {
      const payload: CreateSupportMailboxRequest = {
        name: form.name.trim(),
        handle: normalizeHandle(form.handle),
        icon: form.icon,
        description: form.description.trim() || null,
        linked_team_id: form.linkedTeamId === 'none' ? null : form.linkedTeamId,
        assignment_mode: form.assignmentMode,
        workspace_member_ids: form.workspaceMemberIds,
      };
      await createMailbox.mutateAsync(payload);
      toast.success('Team inbox created');
    }
    setDialogOpen(false);
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

  const toggleMember = (memberId: string, checked: boolean) => {
    setForm((current) => ({
      ...current,
      workspaceMemberIds: checked
        ? [...current.workspaceMemberIds, memberId]
        : current.workspaceMemberIds.filter((id) => id !== memberId),
    }));
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

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-lg overflow-hidden p-0">
          {/* Step indicator */}
          <div className="flex items-center gap-0 border-b px-6 pt-6 pb-4">
            <button
              type="button"
              onClick={() => setStep(1)}
              className="flex items-center gap-2 text-sm font-medium"
            >
              <span className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold transition-colors ${step === 1 ? 'bg-primary text-primary-foreground' : 'bg-primary/10 text-primary'}`}>
                {step > 1 ? <Check className="h-3.5 w-3.5" /> : '1'}
              </span>
              <span className={step === 1 ? 'text-foreground' : 'text-muted-foreground'}>Details</span>
            </button>
            <div className="mx-3 h-px w-8 bg-border" />
            <button
              type="button"
              onClick={() => { if (canProceedToStep2) setStep(2); }}
              className="flex items-center gap-2 text-sm font-medium"
            >
              <span className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold transition-colors ${step === 2 ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'}`}>
                2
              </span>
              <span className={step === 2 ? 'text-foreground' : 'text-muted-foreground'}>Members</span>
            </button>
          </div>

          {/* Step 1: Details */}
          {step === 1 && (
            <div className="px-6 pb-2">
              <DialogHeader className="mb-4">
                <DialogTitle>{editingMailbox ? 'Edit Team Inbox' : 'Create Team Inbox'}</DialogTitle>
                <DialogDescription>Set up the inbox identity and routing preferences.</DialogDescription>
              </DialogHeader>

              <div className="space-y-4">
                {/* Icon + Name row */}
                <div className="flex items-end gap-3">
                  <div className="space-y-1.5">
                    <FieldLabel tip="Appears next to the inbox name in the sidebar and selectors. Pick something that helps your team spot this inbox at a glance.">
                      Icon
                    </FieldLabel>
                    <IconPicker value={form.icon} onChange={(icon) => setForm((current) => ({ ...current, icon }))} />
                  </div>
                  <div className="space-y-1.5 flex-1">
                    <FieldLabel htmlFor="team-inbox-name" tip="The display name shown in the sidebar and conversation headers. Choose something your team will instantly recognize, e.g. &quot;Billing&quot; or &quot;Technical Support&quot;.">
                      Name
                    </FieldLabel>
                    <Input
                      id="team-inbox-name"
                      value={form.name}
                      onChange={(event) => setForm((current) => {
                        const nextName = event.target.value;
                        return {
                          ...current,
                          name: nextName,
                          handle: editingMailbox ? current.handle : normalizeHandle(nextName),
                        };
                      })}
                      placeholder="Technical Support"
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <FieldLabel htmlFor="team-inbox-handle" tip="A unique, URL-safe identifier for this inbox. Used in routing rules and API references. Auto-generated from the name but you can customize it.">
                    Handle
                  </FieldLabel>
                  <div className="relative">
                    <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">#</span>
                    <Input
                      id="team-inbox-handle"
                      value={form.handle}
                      onChange={(event) => setForm((current) => ({ ...current, handle: normalizeHandle(event.target.value) }))}
                      placeholder="technical-support"
                      className="pl-7"
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <FieldLabel htmlFor="team-inbox-description" tip="An internal note for your team explaining what this inbox is for. Not shown to customers.">
                    Description <span className="font-normal text-muted-foreground">(optional)</span>
                  </FieldLabel>
                  <Textarea
                    id="team-inbox-description"
                    value={form.description}
                    onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
                    placeholder="Specialist queue for escalations and technical debugging."
                    rows={3}
                  />
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <FieldLabel tip="Connect this inbox to an existing workspace team. Current members of that team get inbox access automatically, and you can still add extra individual members in the next step.">
                      Linked Team
                    </FieldLabel>
                    <Select value={form.linkedTeamId} onValueChange={(value) => setForm((current) => ({ ...current, linkedTeamId: value }))}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="none">No linked team</SelectItem>
                        {teams.map((team) => (
                          <SelectItem key={team.id} value={team.id}>
                            {team.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <FieldLabel tip="How new conversations are assigned. &quot;Manual&quot; lets agents pick conversations themselves. &quot;Round robin&quot; automatically distributes conversations evenly across online members.">
                      Assignment
                    </FieldLabel>
                    <Select value={form.assignmentMode} onValueChange={(value: 'manual' | 'round_robin') => setForm((current) => ({ ...current, assignmentMode: value }))}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="manual">Manual</SelectItem>
                        <SelectItem value="round_robin">Round robin</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Step 2: Members */}
          {step === 2 && (
            <div className="px-6 pb-2">
              <DialogHeader className="mb-4">
                <DialogTitle>Add Members</DialogTitle>
                <DialogDescription>
                  Choose who can see and respond to conversations in this inbox.
                  {form.linkedTeamId !== 'none' && (
                    <span className="ml-1">Linked team members already have access automatically.</span>
                  )}
                  {form.workspaceMemberIds.length > 0 && (
                    <span className="ml-1 font-medium text-foreground">{form.workspaceMemberIds.length} selected</span>
                  )}
                </DialogDescription>
              </DialogHeader>

              {/* Search */}
              <div className="relative mb-3">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={memberSearch}
                  onChange={(e) => setMemberSearch(e.target.value)}
                  placeholder="Search members..."
                  className="pl-9"
                />
              </div>

              {/* Member list */}
              <ScrollArea className="h-72 rounded-lg border">
                <div className="p-1">
                  {filteredMembers.length === 0 && (
                    <p className="py-8 text-center text-sm text-muted-foreground">No members found</p>
                  )}
                  {filteredMembers.map((member) => {
                    const isSelected = form.workspaceMemberIds.includes(member.id);
                    const displayName = member.display_name || member.email || 'Unknown';
                    return (
                      <label
                        key={member.id}
                        className={`flex items-center gap-3 rounded-md px-2.5 py-2 text-sm cursor-pointer transition-colors ${
                          isSelected ? 'bg-primary/5' : 'hover:bg-muted/60'
                        }`}
                      >
                        <Checkbox
                          checked={isSelected}
                          onCheckedChange={(checked) => toggleMember(member.id, Boolean(checked))}
                        />
                        <Avatar size="sm">
                          <AvatarFallback>{getInitials(displayName)}</AvatarFallback>
                        </Avatar>
                        <div className="min-w-0 flex-1">
                          <p className="truncate font-medium">{displayName}</p>
                          {member.email && member.display_name && (
                            <p className="truncate text-xs text-muted-foreground">{member.email}</p>
                          )}
                        </div>
                        {isSelected && (
                          <Check className="h-4 w-4 shrink-0 text-primary" />
                        )}
                      </label>
                    );
                  })}
                </div>
              </ScrollArea>
            </div>
          )}

          {/* Footer */}
          <DialogFooter className="border-t px-6 py-4">
            {step === 1 ? (
              <>
                <Button variant="outline" onClick={() => setDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={goToStep2} disabled={!canProceedToStep2} className="gap-2">
                  Continue
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </>
            ) : (
              <>
                <Button variant="outline" onClick={() => setStep(1)} className="gap-2 mr-auto">
                  <ChevronLeft className="h-4 w-4" />
                  Back
                </Button>
                <Button
                  onClick={submit}
                  disabled={createMailbox.isPending || updateMailbox.isPending}
                  className="gap-2"
                >
                  <Users className="h-4 w-4" />
                  {editingMailbox ? 'Save Inbox' : 'Create Inbox'}
                </Button>
              </>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
