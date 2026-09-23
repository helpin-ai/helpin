import { useId, useMemo, useState, type FormEvent } from 'react';
import { toast } from 'sonner';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { Button } from '@/components/ui/button';
import { EmailChipInput, classifyEmailChipInput, mergeEmailChips } from '@/components/ui/email-chip-input';
import { Label } from '@/components/ui/label';
import { Copy01Icon, Loading01Icon } from '@/lib/icons';
import { inviteService } from '@/lib/services/inviteService';
import { settingsService } from '@/lib/services/settingsService';
import { OnboardingActions, OnboardingTextButton } from './OnboardingShell';

type InviteStepProps = {
  workspaceId: string;
  /** Teams created during onboarding; non-admin invitees join them. */
  teamIds: string[];
  /** Outbound email isn't set up on this server, so invitations are shared as links. */
  emailUnavailable: boolean;
  onDone: () => void;
};

type InviteLink = { email: string; url: string };

/**
 * Invites teammates. With application email configured, invitations are sent
 * by email; without it (or when a send fails over to a link) each invitation
 * returns a join link the person can copy and share themselves.
 */
export function InviteStep({ workspaceId, teamIds, emailUnavailable, onDone }: InviteStepProps) {
  const id = useId();
  const [emails, setEmails] = useState<string[]>([]);
  const [input, setInput] = useState('');
  const [role, setRole] = useState('member');
  const [sending, setSending] = useState(false);
  const [links, setLinks] = useState<InviteLink[]>([]);
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
    let emailed = 0;
    const created: InviteLink[] = [];
    const failures: { email: string; error: string }[] = [];
    await Promise.all(
      recipients.map(async (email) => {
        const { data, error } = await inviteService.send({ workspace_id: workspaceId, email, role });
        if (error) {
          failures.push({ email, error });
          return;
        }
        if (data?.email_sent === false && data.join_url) created.push({ email, url: data.join_url });
        else emailed++;
        if (data?.id && role !== 'admin' && teamIds.length > 0) {
          await Promise.all(teamIds.map((teamId) => settingsService.addTeamInvitation(workspaceId, teamId, data.id)));
        }
      }),
    );
    setSending(false);
    const failureLines = failures.map((failure) => `${failure.email}: ${failure.error}`).join('\n');
    if (failures.length > 0) {
      toast.error(`${failures.length} invitation${failures.length === 1 ? '' : 's'} couldn’t be created`, { description: failureLines });
    } else if (emailed > 0) {
      toast.success(`${emailed} invitation${emailed === 1 ? '' : 's'} sent`);
    }
    if (created.length > 0) {
      setLinks(created.sort((a, b) => recipients.indexOf(a.email) - recipients.indexOf(b.email)));
      return;
    }
    if (failures.length === 0) onDone();
  };

  if (links.length > 0) return <InviteLinks links={links} onDone={onDone} />;

  return (
    <form onSubmit={(event) => void submit(event)} className="space-y-7">
      {emailUnavailable && (
        <p className="text-sm leading-relaxed text-muted-foreground">
          Email isn’t set up on this server, so Helpin creates an invite link for each person. Share the links yourself.
        </p>
      )}
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
          {sending ? 'Creating…' : emailUnavailable ? 'Create invite links' : 'Send invitations'}
        </Button>
      </OnboardingActions>
    </form>
  );
}

function InviteLinks({ links, onDone }: { links: InviteLink[]; onDone: () => void }) {
  const [copied, setCopied] = useState<string | null>(null);

  const copy = async (link: InviteLink) => {
    try {
      await navigator.clipboard.writeText(link.url);
      setCopied(link.email);
    } catch {
      toast.error('Couldn’t copy the link. Select it and copy it manually.');
    }
  };

  return (
    <div className="space-y-7">
      <p className="text-sm leading-relaxed">
        Send each person their link. It lets them create an account and join this workspace. You can find these links later in Settings → Members.
      </p>
      <ul className="divide-y divide-border border-y border-border">
        {links.map((link) => (
          <li key={link.email} className="flex items-center gap-3 py-3">
            <div className="min-w-0 flex-1">
              <p className="truncate text-[13.5px] font-semibold">{link.email}</p>
              <p className="truncate font-mono text-[11.5px] text-muted-foreground">{link.url}</p>
            </div>
            <OnboardingTextButton
              className="min-h-0 shrink-0 text-[12.5px]"
              aria-label={`Copy invite link for ${link.email}`}
              onClick={() => void copy(link)}
            >
              <span className="inline-flex items-center gap-1.5">
                <Copy01Icon className="h-3.5 w-3.5" aria-hidden="true" />
                {copied === link.email ? 'Copied' : 'Copy link'}
              </span>
            </OnboardingTextButton>
          </li>
        ))}
      </ul>
      <p className="sr-only" role="status" aria-live="polite">{copied ? `Invite link for ${copied} copied` : ''}</p>
      <OnboardingActions>
        <Button type="button" className="w-full sm:w-auto sm:min-w-32" onClick={onDone}>Continue</Button>
      </OnboardingActions>
    </div>
  );
}
