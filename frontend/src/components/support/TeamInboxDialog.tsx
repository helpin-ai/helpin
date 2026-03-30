import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { ArrowRight, Check, ChevronLeft, HelpCircle, Search } from 'lucide-react';
import { toast } from 'sonner';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { IconPicker } from '@/components/ui/icon-picker';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAssignableMembers } from '@/hooks/queries/useWorkspaces';
import { useCreateMailbox, useMailboxMembers, useUpdateMailbox } from '@/hooks/queries/useSupport';
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

const EMPTY_MEMBERS: never[] = [];

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

function FieldLabel({ htmlFor, children, tip }: { htmlFor?: string; children: ReactNode; tip: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <Label htmlFor={htmlFor}>{children}</Label>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="h-3.5 w-3.5 cursor-help text-muted-foreground/60" />
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-56">
          {tip}
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

export function TeamInboxDialog({
  workspaceId,
  open,
  onOpenChange,
  mailbox,
}: {
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mailbox?: SupportMailbox | null;
}) {
  const [step, setStep] = useState<1 | 2>(1);
  const [form, setForm] = useState<MailboxFormState>(DEFAULT_FORM);
  const [memberSearch, setMemberSearch] = useState('');

  const { data: members = [] } = useAssignableMembers(workspaceId);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { data: mailboxMembersData = EMPTY_MEMBERS } = useMailboxMembers(workspaceId, mailbox?.id);
  const mailboxMembers = Array.isArray(mailboxMembersData) ? mailboxMembersData : EMPTY_MEMBERS;
  const createMailbox = useCreateMailbox(workspaceId);
  const updateMailbox = useUpdateMailbox(workspaceId);

  useEffect(() => {
    if (!open) {
      setForm(DEFAULT_FORM);
      setStep(1);
      setMemberSearch('');
      return;
    }
    setForm(buildFormState(mailbox));
    setStep(1);
    setMemberSearch('');
  }, [open, mailbox]);

  useEffect(() => {
    if (!mailbox) return;
    setForm((current) => ({
      ...current,
      workspaceMemberIds: mailboxMembers.map((member) => member.workspace_member_id),
    }));
  }, [mailbox, mailboxMembers]);

  const activeMembers = useMemo(
    () => members.filter((member) => member.status === 'active'),
    [members],
  );

  const filteredMembers = useMemo(() => {
    if (!memberSearch) return activeMembers;
    const q = memberSearch.toLowerCase();
    return activeMembers.filter(
      (member) =>
        member.display_name?.toLowerCase().includes(q) ||
        member.email?.toLowerCase().includes(q),
    );
  }, [activeMembers, memberSearch]);

  const canProceedToStep2 = form.name.trim().length > 0 && form.handle.trim().length > 0;

  const toggleMember = (memberId: string, checked: boolean) => {
    setForm((current) => ({
      ...current,
      workspaceMemberIds: checked
        ? [...current.workspaceMemberIds, memberId]
        : current.workspaceMemberIds.filter((id) => id !== memberId),
    }));
  };

  const goToStep2 = () => {
    if (!form.name.trim()) {
      toast.error('Inbox name is required');
      return;
    }
    if (!form.handle.trim()) {
      toast.error('Inbox handle is required');
      return;
    }
    setStep(2);
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

    if (mailbox) {
      const payload: UpdateSupportMailboxRequest = {
        name: form.name.trim(),
        handle: normalizeHandle(form.handle),
        icon: form.icon,
        description: form.description.trim() || null,
        linked_team_id: form.linkedTeamId === 'none' ? null : form.linkedTeamId,
        assignment_mode: form.assignmentMode,
        workspace_member_ids: form.workspaceMemberIds,
      };
      await updateMailbox.mutateAsync({ mailboxId: mailbox.id, payload });
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

    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="overflow-hidden p-0 sm:max-w-xl">
        <div className="flex items-center gap-0 border-b px-6 pb-4 pt-6">
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
            onClick={() => {
              if (canProceedToStep2) setStep(2);
            }}
            className="flex items-center gap-2 text-sm font-medium"
          >
            <span className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold transition-colors ${step === 2 ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'}`}>
              2
            </span>
            <span className={step === 2 ? 'text-foreground' : 'text-muted-foreground'}>Members</span>
          </button>
        </div>

        {step === 1 && (
          <div className="px-6 pb-2">
            <DialogHeader className="mb-4">
              <DialogTitle>{mailbox ? 'Edit Team Inbox' : 'Create Team Inbox'}</DialogTitle>
              <DialogDescription>Set up the inbox identity and routing preferences.</DialogDescription>
            </DialogHeader>

            <div className="space-y-4">
              <div className="flex items-end gap-3">
                <div className="space-y-1.5">
                  <Label>Icon</Label>
                  <IconPicker value={form.icon} onChange={(icon) => setForm((current) => ({ ...current, icon }))} />
                </div>
                <div className="flex-1 space-y-1.5">
                  <Label htmlFor="team-inbox-name">Name</Label>
                  <Input
                    id="team-inbox-name"
                    value={form.name}
                    onChange={(event) => setForm((current) => {
                      const nextName = event.target.value;
                      return {
                        ...current,
                        name: nextName,
                        handle: mailbox ? current.handle : normalizeHandle(nextName),
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

              <div className="rounded-xl border bg-muted/20 p-3">
                <div className="flex items-end gap-3">
                  <div className="flex-1 space-y-1.5">
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
                  <div className="flex-1 space-y-1.5">
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
          </div>
        )}

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

            <div className="relative mb-3">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={memberSearch}
                onChange={(e) => setMemberSearch(e.target.value)}
                placeholder="Search members..."
                className="pl-9"
              />
            </div>

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
                      className={`flex cursor-pointer items-center gap-3 rounded-md px-2.5 py-2 text-sm transition-colors ${
                        isSelected ? 'bg-primary/5' : 'hover:bg-muted/60'
                      }`}
                    >
                      <Checkbox
                        checked={isSelected}
                        onCheckedChange={(checked) => toggleMember(member.id, Boolean(checked))}
                      />
                      <UserAvatar
                        name={displayName}
                        avatarUrl={member.avatar_url}
                        className="h-6 w-6 border-border/70"
                        fallbackClassName="text-[10px]"
                      />
                      <div className="min-w-0 flex-1">
                        <p className="truncate font-medium">{displayName}</p>
                        {member.email && member.display_name && (
                          <p className="truncate text-xs text-muted-foreground">{member.email}</p>
                        )}
                      </div>
                      {isSelected && <Check className="h-4 w-4 shrink-0 text-primary" />}
                    </label>
                  );
                })}
              </div>
            </ScrollArea>
          </div>
        )}

        <DialogFooter className="border-t px-6 py-4">
          {step === 1 ? (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button onClick={goToStep2} disabled={!canProceedToStep2} className="gap-2">
                Continue
                <ArrowRight className="h-4 w-4" />
              </Button>
            </>
          ) : (
            <>
              <Button variant="outline" onClick={() => setStep(1)} className="mr-auto gap-2">
                <ChevronLeft className="h-4 w-4" />
                Back
              </Button>
              <Button
                onClick={submit}
                disabled={createMailbox.isPending || updateMailbox.isPending}
              >
                {mailbox ? 'Save Changes' : 'Create Inbox'}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
