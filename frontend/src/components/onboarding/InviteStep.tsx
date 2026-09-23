import { useId, useMemo, useState, type FormEvent } from 'react';
import { toast } from 'sonner';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { Button } from '@/components/ui/button';
import { EmailChipInput, classifyEmailChipInput, mergeEmailChips } from '@/components/ui/email-chip-input';
import { Label } from '@/components/ui/label';
import { Loading01Icon } from '@/lib/icons';
import { inviteService } from '@/lib/services/inviteService';
import { settingsService } from '@/lib/services/settingsService';
import { OnboardingActions, OnboardingTextButton } from './OnboardingShell';

type InviteStepProps = {
  workspaceId: string;
  /** Teams created during onboarding; non-admin invitees join them. */
  teamIds: string[];
  /** Outbound email isn't set up on this server, so invitations can't be delivered. */
  emailUnavailable: boolean;
  onDone: () => void;
};

export function InviteStep({ workspaceId, teamIds, emailUnavailable, onDone }: InviteStepProps) {
  if (emailUnavailable) {
    return (
      <div className="space-y-7">
        <p className="text-sm leading-relaxed">
          Email isn’t set up on this server, so invitations can’t be sent yet. You can invite people later from Settings → Members.
        </p>
        <OnboardingActions>
          <Button type="button" className="w-full sm:w-auto sm:min-w-32" onClick={onDone}>Continue</Button>
        </OnboardingActions>
      </div>
    );
  }
  return <InviteForm workspaceId={workspaceId} teamIds={teamIds} onDone={onDone} />;
}

function InviteForm({ workspaceId, teamIds, onDone }: Omit<InviteStepProps, 'emailUnavailable'>) {
  const id = useId();
  const [emails, setEmails] = useState<string[]>([]);
  const [input, setInput] = useState('');
  const [role, setRole] = useState('member');
  const [sending, setSending] = useState(false);
  const recipients = useMemo(() => mergeEmailChips(emails, input), [emails, input]);
  const hasInvalidInput = useMemo(() => classifyEmailChipInput(input).invalid.length > 0, [input]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (recipients.length === 0) {
      onDone();
      return;
    }
    setEmails(recipients);
    setInput('');
    setSending(true);
    let sent = 0;
    const failures: { email: string; error: string }[] = [];
    await Promise.all(
      recipients.map(async (email) => {
        const { data, error } = await inviteService.send({ workspace_id: workspaceId, email, role });
        if (error) {
          failures.push({ email, error });
          return;
        }
        sent++;
        if (data?.id && role !== 'admin' && teamIds.length > 0) {
          await Promise.all(teamIds.map((teamId) => settingsService.addTeamInvitation(workspaceId, teamId, data.id)));
        }
      }),
    );
    setSending(false);
    const failureLines = failures.map((failure) => `${failure.email}: ${failure.error}`).join('\n');
    if (sent > 0 && failures.length > 0) {
      toast.warning(`${sent} of ${recipients.length} invitations sent`, { description: failureLines });
    } else if (sent > 0) {
      toast.success(`${sent} invitation${sent === 1 ? '' : 's'} sent`);
    } else {
      toast.error('Invitations weren’t sent', { description: failureLines });
    }
    onDone();
  };

  return (
    <form onSubmit={(event) => void submit(event)} className="space-y-7">
      <div className="space-y-2">
        <p className="text-sm font-medium" aria-hidden="true">Email addresses</p>
        <EmailChipInput
          ariaLabel="Email addresses"
          placeholder="name@example.com"
          value={emails}
          onValueChange={setEmails}
          inputValue={input}
          onInputValueChange={setInput}
        />
        <p className="text-[12.5px] leading-5 text-muted-foreground">Separate addresses with commas, or paste a list.</p>
      </div>
      <div className="space-y-2">
        <Label htmlFor={`${id}-role`}>Role</Label>
        <Select value={role} onValueChange={setRole}>
          <SelectTrigger id={`${id}-role`} variant="underline" className="w-full px-0.5 sm:w-48">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="member">Member</SelectItem>
            <SelectItem value="admin">Admin</SelectItem>
            <SelectItem value="viewer">Viewer</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <OnboardingActions>
        <OnboardingTextButton onClick={onDone} disabled={sending}>Skip</OnboardingTextButton>
        <Button type="submit" className="w-full sm:w-auto sm:min-w-32" disabled={sending || recipients.length === 0 || hasInvalidInput}>
          {sending && <Loading01Icon className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />}
          {sending ? 'Sending…' : 'Send invitations'}
        </Button>
      </OnboardingActions>
    </form>
  );
}
