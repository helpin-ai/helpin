import { useEffect, useState, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Copy01Icon, Tick01Icon, ArrowDown01Icon } from '@/lib/icons';
import type { SupportEmailRoute } from '@/lib/pmTypes';
import { forwardingStep, forwardingTestState, readForwardingProgress, type ForwardingProvider } from './emailForwardingProgress';

const titles = ['Add the Helpin address', 'Approve the confirmation', 'Enable forwarding in your provider', 'Test delivery'];
function SetupLink({ href, children }: { href: string; children: ReactNode }) {
  return <a href={href} target="_blank" rel="noopener noreferrer" className="font-medium text-primary underline-offset-4 hover:underline">{children} <span aria-hidden="true">↗</span></a>;
}

export function EmailForwardingSetup({ route, busy, confirmationHref, inboxHref, onCopy, onSendTest, onCheck }: {
  route: SupportEmailRoute;
  busy: boolean;
  confirmationHref: string | null;
  inboxHref: string | null;
  onCopy: (address: string) => void | Promise<void>;
  onSendTest: (routeId: string, address: string) => void | Promise<void>;
  onCheck: () => void | Promise<unknown>;
}) {
  const storageKey = `helpin:forwarding-setup:v1:${route.workspace_id}:${route.id}`;
  const [progress, setProgress] = useState(() => readForwardingProgress(storageKey));
  const [source, setSource] = useState(() => progress.source ?? route.source_address ?? '');
  const [expanded, setExpanded] = useState(() => forwardingStep(route, progress.step));
  const [now, setNow] = useState(Date.now);
  const [sending, setSending] = useState(false);
  const [checking, setChecking] = useState(false);
  const [checkError, setCheckError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const step = forwardingStep(route, progress.step);
  const testState = forwardingTestState(route, now);
  const provider = progress.provider;

  useEffect(() => { if (step <= 4) setExpanded(step); }, [step]);
  useEffect(() => {
    try { localStorage.setItem(storageKey, JSON.stringify({ ...progress, source })); } catch { /* Setup still works when browser storage is unavailable. */ }
  }, [storageKey, progress, source]);
  useEffect(() => {
    if (testState !== 'waiting') return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [testState]);

  function advance(next: number) {
    setProgress(current => ({ ...current, step: Math.max(current.step, next) }));
    setExpanded(next);
  }
  async function check() {
    setChecking(true);
    setCheckError(null);
    try { await onCheck(); } catch { setCheckError('Could not refresh the status. Please try again.'); }
    finally { setChecking(false); }
  }
  async function sendTest(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending || busy) return;
    if (source.trim().toLowerCase() === route.inbound_address.toLowerCase()) {
      setError('Enter your existing mailbox address, not the Helpin forwarding address.');
      return;
    }
    setError(null);
    setSending(true);
    try {
      await onSendTest(route.id, source.trim());
      setNow(Date.now());
      advance(4);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'The test could not be sent. Please try again.');
    } finally { setSending(false); }
  }
  const checkButton = <span className="inline-flex flex-wrap items-center gap-2"><Button type="button" size="sm" variant="ghost" disabled={checking} onClick={() => void check()}>{checking ? 'Checking…' : 'Check status'}</Button>{checkError && <span role="alert" className="text-xs text-destructive">{checkError}</span>}</span>;
  const contents = [
    <div className="space-y-3" key="address">
      <p>Add this address as a forwarding destination in the mailbox that receives your customer emails.</p>
      <div className="flex min-w-0 items-center gap-2 rounded-md border bg-muted/40 p-2">
        <code className="min-w-0 flex-1 break-all text-xs text-foreground">{route.inbound_address}</code>
        <Button type="button" size="sm" variant="outline" aria-label="Copy forwarding address" onClick={() => void onCopy(route.inbound_address)}><Copy01Icon className="size-3.5" />Copy</Button>
      </div>
      {provider === 'gmail' ? <p>In Gmail, open <strong>Settings → See all settings → Forwarding and POP/IMAP</strong> (or <strong>Forwarding</strong>), then choose <strong>Add a forwarding address</strong>.</p>
        : provider === 'outlook' ? <p>In Outlook on the web or new Outlook, open <strong>Settings → Mail → Forwarding</strong> and enter this address. Your organization may restrict external forwarding.</p>
          : <p>Find forwarding or mail routing in your provider’s settings and add the Helpin address as the destination.</p>}
      {provider !== 'other' && <SetupLink href={provider === 'gmail' ? 'https://support.google.com/mail/answer/10957?hl=en' : 'https://support.microsoft.com/en-us/outlook/mail/turn-automatic-forwarding-on-or-off-in-outlook'}>View provider instructions</SetupLink>}
      <div><Button type="button" size="sm" onClick={() => advance(2)}>I’ve added the address</Button></div>
    </div>,
    <div className="space-y-3" key="confirmation">
      {confirmationHref ? <><p className="flex items-center gap-1.5 text-emerald-700 dark:text-emerald-400"><Tick01Icon className="size-3.5" />Confirmation email received</p><p>Open the email in Helpin and follow your provider’s verification link or instructions.</p><SetupLink href={confirmationHref}>Open confirmation email in Helpin</SetupLink></>
        : <><p>{provider === 'outlook' ? 'Outlook usually does not require a confirmation email. If your provider asks you to verify the destination, look for its email in Helpin.' : 'Waiting for your provider’s confirmation email. It will appear here when Helpin receives it.'}</p>{inboxHref && <SetupLink href={inboxHref}>Open {route.mailbox_id ? 'Team Inbox' : 'Shared Inbox'}</SetupLink>} {checkButton}</>}
      <p className="text-xs">Helpin can detect the email’s arrival, but cannot see whether you approve it in your provider.</p>
      <div className="flex flex-wrap gap-2"><Button type="button" size="sm" onClick={() => advance(3)}>I’ve confirmed</Button><Button type="button" size="sm" variant="ghost" onClick={() => advance(3)}>My provider doesn’t require confirmation</Button></div>
    </div>,
    <div className="space-y-3" key="enable">
      {provider === 'gmail' ? <><p>Return to Gmail settings and <strong>refresh the page</strong>. Open the Forwarding tab, select <strong>Forward a copy of incoming mail to</strong>, and choose your Helpin address.</p><p>Choose what happens to Gmail’s copy, then click <strong>Save Changes</strong>. We recommend keeping a copy in your Gmail inbox.</p></>
        : provider === 'outlook' ? <p>Turn on <strong>Enable forwarding</strong>, check the Helpin address, and click <strong>Save</strong>. Select <strong>Keep a copy of forwarded messages</strong> if you want a copy in Outlook.</p>
          : <p>Enable automatic forwarding to the Helpin address and save your provider’s settings. Adding or confirming the address alone may not enable forwarding.</p>}
      <p className="text-xs">If you use a forwarding rule, make sure the delivery test will match it. Existing emails are not imported by this setup.</p>
      <Button type="button" size="sm" onClick={() => advance(4)}>I’ve enabled forwarding</Button>
    </div>,
    <div className="space-y-3" key="test">
      <p>We’ll send an email to your existing mailbox and check that it reaches this Helpin inbox through forwarding.</p>
      <form onSubmit={event => void sendTest(event)} className="space-y-3">
        <div><Label htmlFor={`forwarding-source-${route.id}`}>Your existing mailbox address</Label><Input className="mt-1.5" id={`forwarding-source-${route.id}`} type="email" required value={source} onChange={event => { setSource(event.target.value); setError(null); }} placeholder="support@company.com" disabled={sending || busy} /><p className="mt-1 text-xs">Enter the address customers email, not your Helpin forwarding address.</p></div>
        <div className="flex flex-wrap items-center gap-2"><Button type="submit" size="sm" disabled={sending || busy || !source.trim() || testState === 'waiting'}>{sending ? 'Sending test…' : testState === 'waiting' ? 'Waiting for delivery…' : route.verification_sent_at || error ? 'Send another test' : 'Send test email'}</Button>{checkButton}</div>
      </form>
      {testState === 'waiting' && <p role="status" className="text-xs">Test sent. Checking automatically for delivery. You can leave this page and return later.</p>}
      {testState === 'delayed' && <div role="status" className="rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-950 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-100">The test hasn’t arrived yet. Check that forwarding is enabled and saved, the source address is correct, and spam filters or forwarding rules haven’t blocked the test. Then send another test. Your provider’s administrator may need to allow external forwarding.</div>}
      {(error || route.forwarding_last_error) && <p role="alert" className="text-sm text-destructive">{error || route.forwarding_last_error}</p>}
    </div>,
  ];

  return <section className="mt-3 overflow-hidden rounded-lg border bg-card sm:ml-11" aria-label="Email forwarding setup">
    <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3"><div><h3 className="text-sm font-medium">Set up email forwarding</h3><p className="mt-0.5 text-xs text-muted-foreground">Step {Math.min(step, 4)} of 4 · Delivery is verified in the final step</p></div><div className="flex items-center gap-2"><Label htmlFor={`provider-${route.id}`} className="text-xs">Provider</Label><select id={`provider-${route.id}`} value={provider} onChange={event => setProgress(current => ({ ...current, provider: event.target.value as ForwardingProvider }))} className="h-8 rounded-md border bg-background px-2 text-xs"><option value="gmail">Gmail</option><option value="outlook">Outlook</option><option value="other">Other</option></select></div></div>
    <div className="divide-y">{titles.map((title, index) => {
      const number = index + 1;
      const complete = number < step;
      const open = number === expanded;
      return <div key={title}><button type="button" aria-expanded={open} aria-controls={`forwarding-${route.id}-${number}`} onClick={() => setExpanded(open ? 0 : number)} className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"><span className={`flex size-6 shrink-0 items-center justify-center rounded-full text-xs ${complete ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' : number === step ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'}`}>{complete ? <Tick01Icon className="size-3.5" aria-label="Complete" /> : number}</span><span className="flex-1 text-sm font-medium">{title}</span><ArrowDown01Icon aria-hidden="true" className={`size-3.5 text-muted-foreground transition-transform motion-reduce:transition-none ${open ? 'rotate-180' : ''}`} /></button>{open && <div id={`forwarding-${route.id}-${number}`} className="px-4 pb-4 text-sm leading-relaxed text-muted-foreground sm:pl-13">{contents[index]}</div>}</div>;
    })}</div>
  </section>;
}
