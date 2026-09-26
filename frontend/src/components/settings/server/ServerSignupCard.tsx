import { useState, type FormEvent } from 'react';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Card, CardContent } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { QuietPrimaryAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useSignupPolicy, useUpdateSignupPolicy } from '@/hooks/queries/useInstance';
import type { SignupMode, SignupPolicy } from '@/lib/instanceTypes';
import { parseDomains } from '@/lib/signupPolicy';

const MODES: { value: SignupMode; label: string; help: string }[] = [
  { value: 'invite_only', label: 'Invite only', help: 'People join when a workspace admin invites them.' },
  { value: 'domains', label: 'Approved email domains', help: 'Anyone with an address on these domains can sign up after confirming their email. Others need an invite.' },
  { value: 'open', label: 'Anyone', help: 'Anyone who can reach this server can create an account.' },
];

/** Settings → Signup & admins: who may create an account on this server. */
export function ServerSignupCard({ slug }: { slug: string }) {
  const policy = useSignupPolicy();

  if (policy.isLoading) return <Card className="rounded-lg border-border/70 py-0"><CardContent className="space-y-3 p-4"><Skeleton className="h-5 w-40" /><Skeleton className="h-8 w-72 max-w-full" /></CardContent></Card>;
  if (policy.isError || !policy.data) {
    return <Card className="rounded-lg border-border/70 py-0"><CardContent className="p-4 text-sm text-quiet-text-tertiary">Couldn’t load the signup policy. Reload the page to try again.</CardContent></Card>;
  }
  // Re-mount the form whenever the saved policy changes so it starts from it.
  const current = policy.data;
  return <ServerSignupForm key={`${current.mode}:${current.allowed_domains.join(',')}:${current.app_email_configured}`} slug={slug} current={current} />;
}

function ServerSignupForm({ slug, current }: { slug: string; current: SignupPolicy }) {
  const update = useUpdateSignupPolicy();
  const [mode, setMode] = useState<SignupMode>(current.mode);
  const [domains, setDomains] = useState(current.allowed_domains.join(', '));

  const parsedDomains = parseDomains(domains);
  const dirty = mode !== current.mode || (mode === 'domains' && parsedDomains.join(',') !== current.allowed_domains.join(','));
  const domainsBlocked = mode === 'domains' && !current.app_email_configured;
  const selected = MODES.find((option) => option.value === mode) ?? MODES[0];

  const save = async (event: FormEvent) => {
    event.preventDefault();
    try {
      await update.mutateAsync({ mode, allowed_domains: mode === 'domains' ? parsedDomains : [] });
      toast.success('Signup policy saved');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Couldn’t save the signup policy.');
    }
  };

  return (
    <Card className="rounded-lg border-border/70 py-0">
      <CardContent className="p-4">
        <form onSubmit={(event) => void save(event)} className="space-y-4" aria-labelledby="server-signup-title">
          <div>
            <h2 id="server-signup-title" className="text-sm font-semibold text-quiet-text-primary">Signup</h2>
            <p className="mt-0.5 text-[12.5px] text-quiet-text-tertiary">Invitations always work, whichever option you choose.</p>
          </div>
          {current.recommend_invite_only && (
            <div className="relative py-1 pl-4" role="note">
              <span aria-hidden="true" className="absolute inset-y-0 left-0 w-[3px] bg-quiet-accent" />
              <p className="text-sm font-medium text-quiet-text-primary">Anyone who can reach this server can sign up</p>
              <p className="mt-0.5 text-[12.5px] text-quiet-text-tertiary">Switch to invite only unless this server is meant to be public.</p>
            </div>
          )}
          <div className="max-w-md space-y-1.5">
            <Label htmlFor="server-signup-mode">Who can create an account</Label>
            <Select value={mode} onValueChange={(value) => setMode(value as SignupMode)}>
              <SelectTrigger id="server-signup-mode" aria-label="Who can create an account" variant="underline" className="w-full px-0.5">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {MODES.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
              </SelectContent>
            </Select>
            <p className="text-[12.5px] text-quiet-text-tertiary">{selected.help}</p>
          </div>
          {mode === 'domains' && (
            <div className="max-w-md space-y-1.5">
              <Label htmlFor="server-signup-domains">Email domains</Label>
              <QuietUnderlineInput
                id="server-signup-domains"
                value={domains}
                onChange={(event) => setDomains(event.target.value)}
                placeholder="example.com, example.org"
                autoComplete="off"
              />
              <p className="text-[12.5px] text-quiet-text-tertiary">Separate domains with commas. Subdomains must be listed on their own.</p>
              {domainsBlocked && (
                <p className="text-[12.5px] text-quiet-accent" role="alert">
                  New accounts confirm their email first, so set up email in{' '}
                  <Link to="/w/$slug/settings/system-status" params={{ slug }} className="underline underline-offset-2">System status</Link>{' '}
                  before choosing this option.
                </p>
              )}
            </div>
          )}
          <QuietPrimaryAction type="submit" disabled={!dirty || update.isPending || domainsBlocked || (mode === 'domains' && parsedDomains.length === 0)}>
            {update.isPending ? 'Saving…' : 'Save signup policy'}
          </QuietPrimaryAction>
        </form>
      </CardContent>
    </Card>
  );
}
