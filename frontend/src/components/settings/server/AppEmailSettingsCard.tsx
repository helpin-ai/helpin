import { useState, type FormEvent, type ReactNode } from 'react';
import { toast } from 'sonner';
import { Card, CardContent } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { SetupResultMessage, type SetupResult } from '@/components/setup/CapabilityActions';
import { useAppEmailSettings, useClearAppEmailSettings, useSaveAppEmailSettings, useSendServerTestEmail } from '@/hooks/queries/useInstance';
import type { AppEmailSettings, AppEmailSettingsUpdate } from '@/lib/instanceTypes';
import { Loading01Icon, Mail01Icon } from '@/lib/icons';

type TLSMode = AppEmailSettingsUpdate['tls_mode'];

const TLS_OPTIONS: { value: TLSMode; label: string }[] = [
  { value: 'starttls', label: 'STARTTLS (usually port 587)' },
  { value: 'tls', label: 'TLS (usually port 465)' },
  { value: 'none', label: 'None (trusted local relay)' },
];

type Draft = { host: string; port: string; username: string; password: string; from: string; tls_mode: TLSMode };

function draftFrom(settings: AppEmailSettings): Draft {
  return {
    host: settings.host,
    port: settings.port ? String(settings.port) : '587',
    username: settings.username,
    password: '',
    from: settings.from,
    tls_mode: (settings.tls_mode || 'starttls') as TLSMode,
  };
}

/**
 * System status → Application email: SMTP settings for invitations, password
 * resets and notifications. Server configuration (SMTP_* or POSTMARK_*) takes
 * precedence and is shown read-only. The password is write-only.
 */
export function AppEmailSettingsCard() {
  const settings = useAppEmailSettings();

  if (settings.isLoading) {
    return <Card className="rounded-lg border-border/70 py-0"><CardContent className="space-y-3 p-4"><Skeleton className="h-5 w-40" /><Skeleton className="h-24 w-full" /></CardContent></Card>;
  }
  if (settings.isError || !settings.data) {
    return <Card className="rounded-lg border-border/70 py-0"><CardContent className="p-4 text-sm text-quiet-text-tertiary">Couldn’t load the email settings. Reload the page to try again.</CardContent></Card>;
  }
  // Re-mount the form whenever the saved settings change so it starts from them.
  return <AppEmailSettingsForm key={JSON.stringify(settings.data)} current={settings.data} />;
}

function AppEmailSettingsForm({ current }: { current: AppEmailSettings }) {
  const save = useSaveAppEmailSettings();
  const clear = useClearAppEmailSettings();
  const sendTest = useSendServerTestEmail();
  const [draft, setDraft] = useState<Draft>(() => draftFrom(current));
  const [testResult, setTestResult] = useState<SetupResult | null>(null);

  const readOnly = !current.editable;
  const configured = current.source !== 'none' && !current.problem;
  const set = (patch: Partial<Draft>) => setDraft({ ...draft, ...patch });

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setTestResult(null);
    try {
      await save.mutateAsync({
        host: draft.host.trim(),
        port: Number(draft.port) || 0,
        username: draft.username.trim(),
        // Blank keeps the saved password; it is never sent back to the browser.
        password: draft.password === '' ? null : draft.password,
        from: draft.from.trim(),
        tls_mode: draft.tls_mode,
      });
      toast.success('Email settings saved. Send a test email to confirm delivery.');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Couldn’t save the email settings.');
    }
  };

  const test = async () => {
    setTestResult(null);
    try {
      const outcome = await sendTest.mutateAsync();
      setTestResult(outcome.ok
        ? { tone: 'positive', message: outcome.recipient ? `Test email sent to ${outcome.recipient}. Check that it arrived.` : 'Test email sent. Check that it arrived.' }
        : { tone: 'negative', message: outcome.error ?? 'The mail server did not accept the message.' });
    } catch (error) {
      setTestResult({ tone: 'negative', message: `Couldn’t send the test email: ${error instanceof Error ? error.message : 'try again.'}` });
    }
  };

  return (
    <Card className="rounded-lg border-border/70 py-0" aria-labelledby="app-email-title">
      <CardContent className="space-y-4 p-4">
        <div>
          <h2 id="app-email-title" className="text-sm font-semibold text-quiet-text-primary">Application email</h2>
          <p className="mt-0.5 text-[12.5px] text-quiet-text-tertiary">
            {readOnly
              ? 'Set by the server’s configuration. Change the SMTP_* or POSTMARK_* variables to update it.'
              : 'Sends invitations, password resets and notifications. Without it, invitations are shared as links.'}
          </p>
        </div>
        {current.problem && <p className="text-[12.5px] text-quiet-accent" role="alert">{current.problem}</p>}
        {readOnly && current.provider === 'postmark' ? (
          <dl className="grid max-w-md grid-cols-[7rem_1fr] gap-y-1.5 text-sm">
            <dt className="text-quiet-text-tertiary">Provider</dt><dd className="text-quiet-text-primary">Postmark</dd>
            <dt className="text-quiet-text-tertiary">From</dt><dd className="truncate text-quiet-text-primary">{current.from}</dd>
          </dl>
        ) : (
          <form onSubmit={(event) => void submit(event)} className="space-y-4">
            <fieldset disabled={readOnly || save.isPending} className="grid max-w-2xl gap-x-6 gap-y-4 sm:grid-cols-2">
              <Field id="smtp-host" label="SMTP server">
                <QuietUnderlineInput id="smtp-host" value={draft.host} onChange={(e) => set({ host: e.target.value })} placeholder="smtp.example.com" autoComplete="off" required />
              </Field>
              <Field id="smtp-port" label="Port">
                <QuietUnderlineInput id="smtp-port" inputMode="numeric" value={draft.port} onChange={(e) => set({ port: e.target.value.replace(/\D/g, '') })} placeholder="587" required />
              </Field>
              <Field id="smtp-username" label="Username">
                <QuietUnderlineInput id="smtp-username" value={draft.username} onChange={(e) => set({ username: e.target.value })} placeholder="Leave empty for a relay without sign-in" autoComplete="off" />
              </Field>
              <Field id="smtp-password" label="Password">
                <QuietUnderlineInput
                  id="smtp-password"
                  type="password"
                  value={draft.password}
                  onChange={(e) => set({ password: e.target.value })}
                  placeholder={current.password_set ? 'Saved — leave empty to keep it' : 'SMTP password'}
                  autoComplete="new-password"
                />
              </Field>
              <Field id="smtp-from" label="Send as">
                <QuietUnderlineInput id="smtp-from" value={draft.from} onChange={(e) => set({ from: e.target.value })} placeholder="Helpin <helpin@example.com>" autoComplete="off" required />
              </Field>
              <Field id="smtp-tls" label="Security">
                <Select value={draft.tls_mode} onValueChange={(value) => set({ tls_mode: value as TLSMode })} disabled={readOnly || save.isPending}>
                  <SelectTrigger id="smtp-tls" aria-label="Security" variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger>
                  <SelectContent>{TLS_OPTIONS.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
                </Select>
              </Field>
            </fieldset>
            {!readOnly && (
              <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                <QuietPrimaryAction type="submit" disabled={save.isPending}>{save.isPending ? 'Saving…' : 'Save'}</QuietPrimaryAction>
                {current.source === 'database' && (
                  <QuietTextAction type="button" disabled={clear.isPending} onClick={() => void clear.mutateAsync().then(() => toast.success('Email settings removed'), (error: unknown) => toast.error(error instanceof Error ? error.message : 'Couldn’t remove the email settings.'))}>
                    Remove settings
                  </QuietTextAction>
                )}
              </div>
            )}
          </form>
        )}
        {configured && (
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-quiet-divider-light pt-4">
            <QuietTextAction type="button" disabled={sendTest.isPending} onClick={() => void test()}>
              {sendTest.isPending ? <Loading01Icon className="mr-1 size-4 animate-spin" aria-hidden="true" /> : <Mail01Icon className="mr-1 size-4" aria-hidden="true" />}
              {sendTest.isPending ? 'Sending…' : 'Send test email'}
            </QuietTextAction>
            <span className="text-[12px] text-quiet-text-tertiary">Sends to the email on your account.</span>
          </div>
        )}
        <SetupResultMessage result={testResult} />
      </CardContent>
    </Card>
  );
}

function Field({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="min-w-0 space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
    </div>
  );
}
