import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode, type Ref } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { AskAgentWorkAnimation } from '@/components/agents/AskAgentWorkAnimation';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useTitle } from '@/hooks/useTitle';
import { buildLoginPathForRedirect } from '@/lib/authRedirect';
import { Alert01Icon, ArrowDown01Icon, Loading01Icon, Tick01Icon } from '@/lib/icons';
import { savePendingSupportSetup } from '@/lib/pendingSupportSetup';
import { selfSignupAllowed } from '@/lib/services/authService';
import type { SupportQuickstartDetection } from '@/lib/services/supportQuickstartService';
import { formatDomainList, INVITE_ONLY_MESSAGE } from '@/lib/signupPolicy';
import { cn } from '@/lib/utils';
import { siteLabel, websiteDisplayName } from '@/lib/workspaceOnboardingFlow';
import { ensureAuthConfiguration, useAuthStore } from '@/stores/authStore';
import { OnboardingShell } from './OnboardingShell';
import { BrandMark, SupportAgentPreview } from './SupportAgentPreview';
import { WebsiteForm } from './SupportWebsiteStep';
import { prefersReducedMotion, useSupportPreview, type SupportPreviewRun } from './useSupportPreview';

// Signing in instead of up continues with the saved choices.
const SIGN_IN_THEN_SETUP = buildLoginPathForRedirect('/onboarding') as '/login';

/**
 * The public start of support onboarding. From a website it shows, step by
 * step as the server does the work, the brand and help center it found and
 * the AI agent answering real customer questions from them, all before an
 * account exists. The account is asked for only when the person chooses to
 * put the agent live; what was found is kept so setup starts right after
 * signup.
 */
export function SupportStart({ website }: { website?: string }) {
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  useTitle(website ? `Your AI agent for ${websiteDisplayName(website)}` : 'Set up support');
  const headerAction = user ? null : (
    <Link to={SIGN_IN_THEN_SETUP} className="public-page-back rounded-sm underline-offset-4 hover:text-foreground hover:underline">Sign in</Link>
  );

  if (!website) {
    return (
      <OnboardingShell
        steps={[]}
        current={undefined}
        title="Meet your AI support agent"
        description="Enter your website. In about 30 seconds you’ll see your agent answer real customer questions from your own help center."
        headerAction={headerAction}
      >
        <WebsiteForm defaultWebsite="" onSubmit={(next) => void navigate({ to: '/start', search: { website: next } })} />
      </OnboardingShell>
    );
  }
  return (
    <OnboardingShell
      steps={[]}
      current={undefined}
      title="Meet your AI support agent"
      description={`Built from ${websiteDisplayName(website)} while you watch. Nothing goes live until you say so.`}
      headerAction={headerAction}
    >
      <SupportPreview key={website} website={website} />
    </OnboardingShell>
  );
}

function SupportPreview({ website }: { website: string }) {
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  const host = websiteDisplayName(website);
  const [name, setName] = useState<string | null>(null);
  const [addedHelpCenter, setAddedHelpCenter] = useState<string>();
  const [signingUp, setSigningUp] = useState(false);
  const { run, retry } = useSupportPreview(website, addedHelpCenter);
  const { site, helpCenter } = run;
  const answersRef = useRef<HTMLLIElement>(null);
  const answered = Boolean(run.preview ?? run.answersFailed);
  const [folded, setFolded] = useState<StepView['id']>();
  // Steps the person opened or closed themselves stay that way.
  const [chosen, setChosen] = useState<Partial<Record<StepView['id'], boolean>>>({});
  const steps = previewSteps(run, host);
  // The newest result is shown in full for a moment, then folds into its title.
  const latest = steps.findLast((step) => step.state !== 'pending' && step.state !== 'working')?.id;
  useEffect(() => {
    if (!latest) return;
    const timer = setTimeout(() => setFolded(latest), RESULT_HOLD_MS);
    return () => clearTimeout(timer);
  }, [latest]);

  // The answers are the point of the page; bring them into view when they arrive.
  useEffect(() => {
    if (!answered) return;
    answersRef.current?.scrollIntoView({ behavior: prefersReducedMotion() ? 'auto' : 'smooth', block: 'nearest' });
  }, [answered]);

  if (!site && run.status === 'stopped') {
    return (
      <div className="space-y-6">
        <p role="alert" className="flex gap-2 text-[13px] leading-5 text-amber-900 dark:text-amber-200">
          <Alert01Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          {run.error}
        </p>
        <WebsiteForm
          defaultWebsite={host}
          onSubmit={(next) => (next === website ? retry() : void navigate({ to: '/start', search: { website: next } }))}
        />
      </div>
    );
  }

  const companyName = (name ?? site?.company_name ?? '').trim() || host;
  const helpUrl = helpCenter?.url ?? '';
  const putLive = () => {
    if (!site) return;
    const foundHelpCenter = helpUrl ? { url: helpUrl, platform: helpCenter?.counts ? 'intercom' as const : 'website' as const } : undefined;
    savePendingSupportSetup({
      detection: { ...site, company_name: companyName, help_center: foundHelpCenter },
      companyName,
      helpCenterUrl: helpUrl || undefined,
      // The agent answers what the pages cover; drafts-only is a setting for later.
      aiMode: 'answer',
    });
    if (user) {
      void navigate({ to: '/onboarding' });
      return;
    }
    setSigningUp(true);
  };

  const working = steps.find((step) => step.state === 'working');
  const autoOpen = (step: StepView, index: number) =>
    index === steps.length - 1 || step.state === 'working' || step.id === 'answers'
    // With no help center found, the step is asking for its address.
    || (step.id === 'help_center' && helpCenter !== undefined && !helpCenter.url)
    || (step.id === latest && folded !== latest);
  // Offered as soon as the help center is known; the answers can still be arriving.
  const canPutLive = Boolean(site) && (Boolean(helpCenter) || run.status !== 'running');

  return (
    <div className="space-y-8">
      <p role="status" className="sr-only">{working ? working.title : 'Finished'}</p>
      <ol>
        {steps.map((step, index) => (
          <Step
            key={step.id}
            ref={step.id === 'answers' ? answersRef : undefined}
            step={step}
            last={index === steps.length - 1}
            log={run.logs[step.id]}
            open={chosen[step.id] ?? autoOpen(step, index)}
            onOpenChange={step.id === 'answers' ? undefined : (open) => setChosen((previous) => ({ ...previous, [step.id]: open }))}
            summary={stepSummary(step, companyName, helpCenter)}
          >
            {step.id === 'site' && (site
              ? <BrandCard site={site} name={name ?? site.company_name} onNameChange={setName} />
              : <BrandSkeleton />)}
            {step.id === 'help_center' && helpCenter && (
              <HelpCenterFound host={host} found={helpCenter} onAdd={setAddedHelpCenter} />
            )}
            {step.id === 'articles' && run.articles && run.articles.length > 0 && (
              <ul className="space-y-1 text-[13px] leading-5 text-muted-foreground">
                {run.articles.map((article, index) => (
                  <li
                    key={article.url}
                    className="flex gap-2 duration-300 animate-in fade-in slide-in-from-left-1 fill-mode-backwards motion-reduce:animate-none"
                    style={{ animationDelay: `${index * 70}ms` }}
                  >
                    <Tick01Icon className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" aria-hidden="true" />
                    <a href={article.url} target="_blank" rel="noreferrer noopener" className="min-w-0 truncate underline-offset-2 hover:text-foreground hover:underline">{article.title}</a>
                  </li>
                ))}
              </ul>
            )}
            {step.id === 'answers' && site && (run.preview || run.answersFailed) && (
              <div className="space-y-3">
                {run.answersFailed && (
                  <p className="text-[12.5px] leading-5 text-muted-foreground">{run.answersFailed} You can still put it live; it answers once your content is in.</p>
                )}
                {run.preview && run.preview.answers.length === 0 && (
                  <p className="text-[12.5px] leading-5 text-muted-foreground">
                    It will answer from {helpCenter?.counts ? `your ${helpCenter.counts.articles} articles` : host} once it’s live. Ask it something to see how it hands questions to your team.
                  </p>
                )}
                {run.preview && (
                  <SupportAgentPreview
                    site={site}
                    companyName={companyName}
                    helpUrl={helpUrl}
                    answers={run.preview.answers}
                    gaps={run.preview.gaps}
                    articleCount={helpCenter?.counts?.articles}
                  />
                )}
              </div>
            )}
          </Step>
        ))}
      </ol>

      {run.status === 'running' && !answered && <WhileYouWait />}
      {site && run.status === 'stopped' && (
        <p role="alert" className="text-[13px] leading-5 text-muted-foreground">
          {run.error} <button type="button" className="underline underline-offset-2 hover:text-foreground" onClick={retry}>Try again</button>
        </p>
      )}

      {canPutLive && (signingUp ? (
        <InlineSignup host={host} onSignedUp={() => void navigate({ to: '/onboarding' })} />
      ) : (
        <div className="sticky bottom-0 -mx-4 border-t bg-background/95 px-4 py-4 backdrop-blur duration-300 animate-in fade-in slide-in-from-bottom-2 supports-[backdrop-filter]:bg-background/80 motion-reduce:animate-none sm:static sm:mx-0 sm:bg-transparent sm:px-0 sm:pb-0 sm:pt-6 sm:backdrop-blur-none">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            {/* On a phone the bar stays on screen, so it holds the button alone. */}
            <p className="hidden text-[12.5px] leading-5 text-muted-foreground sm:block">
              Your inbox, chat widget and agent, set up in one step. You can switch the agent to drafting replies for your team in settings.
            </p>
            <Button size="lg" className="w-full shrink-0 sm:w-auto sm:min-w-56" onClick={putLive}>Put this on {host}</Button>
          </div>
        </div>
      ))}
    </div>
  );
}

type StepState = 'pending' | 'working' | 'done' | 'note' | 'failed';
type StepView = {
  id: 'site' | 'help_center' | 'articles' | 'answers';
  state: StepState;
  title: string;
  /** The number in the title, counted up when the step finishes. */
  count?: number;
};

// How long a step's result stays open before it folds into its title.
const RESULT_HOLD_MS = 2600;

/** Each step of the preview with its current state, titled by its outcome once it has one. */
function previewSteps(run: SupportPreviewRun, host: string): StepView[] {
  const { site, helpCenter, articles, preview } = run;
  // A step the run never reached stays unfinished instead of spinning forever.
  const reached = (ready: boolean): StepState => (!ready ? 'pending' : run.status === 'running' ? 'working' : 'note');
  const steps: StepView[] = [];

  if (!site) steps.push({ id: 'site', state: 'working', title: `Reading ${host}` });
  else if (site.website_readable) steps.push({ id: 'site', state: 'done', title: `Read ${host}` });
  else steps.push({ id: 'site', state: 'note', title: `Couldn’t open ${host}` });

  if (!helpCenter) steps.push({ id: 'help_center', state: reached(Boolean(site)), title: site ? 'Finding your help center' : 'Find your help center' });
  else if (helpCenter.counts) steps.push({ id: 'help_center', state: 'done', title: `Found ${count(helpCenter.counts.articles, 'help article')}`, count: helpCenter.counts.articles });
  else if (helpCenter.url) steps.push({ id: 'help_center', state: 'done', title: 'Found your help center' });
  else steps.push({ id: 'help_center', state: 'note', title: 'No help center found' });

  // Without AI on this server there is nothing to read for or answer.
  if (preview && !preview.available) return steps;

  const unit = helpCenter && !helpCenter.counts ? 'page' : 'article';
  // When nothing could be read, the answers step says so; a step of its own would repeat it.
  if (!articles) steps.push({ id: 'articles', state: reached(Boolean(helpCenter)), title: helpCenter ? `Reading your ${unit}s` : 'Read your articles' });
  else if (articles.length > 0) steps.push({ id: 'articles', state: 'done', title: `Read ${count(articles.length, unit)}`, count: articles.length });

  if (run.answersFailed) steps.push({ id: 'answers', state: 'failed', title: 'Your agent couldn’t answer just now' });
  else if (!preview) steps.push({ id: 'answers', state: reached(Boolean(articles)), title: articles ? 'Answering customer questions' : 'Answer customer questions' });
  else if (preview.answers.length === 0) steps.push({ id: 'answers', state: 'note', title: 'Your agent is ready to learn' });
  else {
    const gaps = preview.gaps.length > 0 ? `, found ${preview.gaps.length} without an article` : '';
    steps.push({ id: 'answers', state: 'done', title: `Answered ${count(preview.answers.length, 'customer question')}${gaps}`, count: preview.answers.length });
  }
  return steps;
}

/** What a folded step still says next to its title. */
function stepSummary(step: StepView, companyName: string, helpCenter: SupportPreviewRun['helpCenter']) {
  if (step.id === 'site') return companyName;
  if (step.id === 'help_center' && helpCenter?.url) {
    return helpCenter.counts ? `${siteLabel(helpCenter.url)} · ${count(helpCenter.counts.collections, 'collection')}` : siteLabel(helpCenter.url);
  }
  return undefined;
}

function count(n: number, noun: string) {
  return `${n} ${noun}${n === 1 ? '' : 's'}`;
}

/**
 * One step on the timeline: its marker on the line, its title, the work in
 * progress, then what it found. A finished step with something to show can be
 * folded to its title and opened again.
 */
function Step({ ref, step, last, log, open, onOpenChange, summary, children }: {
  ref?: Ref<HTMLLIElement>;
  step: StepView;
  last: boolean;
  log?: string[];
  open: boolean;
  /** Left out for a step that never folds. */
  onOpenChange?: (open: boolean) => void;
  summary?: string;
  children?: ReactNode;
}) {
  const { state } = step;
  const id = useId();
  const settled = state !== 'pending' && state !== 'working';
  const foldable = settled && Boolean(children) && onOpenChange !== undefined;
  const large = open && state !== 'pending';
  const title = (
    // Keyed so the number counts up when the step finishes.
    <StepTitle key={step.count ?? 'none'} step={step} />
  );
  const heading = cn(
    'transition-[font-size,color] duration-300 motion-reduce:transition-none',
    large ? 'text-[17px] font-semibold leading-6 tracking-[-0.01em]' : 'text-[15px] font-medium leading-6',
    state === 'pending' && 'font-normal text-muted-foreground',
  );
  return (
    <li ref={ref} className={cn('relative scroll-mb-28 pl-10', !last && 'pb-6')}>
      {!last && (
        <span aria-hidden="true" className={cn('absolute bottom-0 left-[11.5px] top-7 w-px transition-colors duration-500', settled && state === 'done' ? 'bg-emerald-600/40' : 'bg-border')} />
      )}
      <span aria-hidden="true" className="absolute left-0 top-0 flex h-6 w-6 items-center justify-center bg-background">
        {state === 'done' && (
          <>
            {/* One ring that spreads from the tick the moment the step finishes. */}
            <span className="absolute inset-0 animate-ping rounded-full bg-emerald-500/40 [animation-iteration-count:1] [animation-fill-mode:forwards] motion-reduce:hidden" />
            <span className="relative flex h-6 w-6 items-center justify-center rounded-full bg-emerald-600 text-white duration-500 ease-out animate-in zoom-in-50 spin-in-[-30deg] motion-reduce:animate-none">
              <Tick01Icon className="h-4 w-4" strokeWidth={2.5} />
            </span>
          </>
        )}
        {state === 'working' && <AskAgentWorkAnimation className="h-5 w-5 motion-reduce:animate-none" />}
        {state === 'pending' && <span className="h-2.5 w-2.5 rounded-full border-[1.5px] border-muted-foreground/40" />}
        {state === 'note' && <span className="h-2.5 w-2.5 rounded-full bg-muted-foreground/60" />}
        {state === 'failed' && <span className="h-2.5 w-2.5 rounded-full bg-amber-500" />}
      </span>
      {foldable ? (
        <h2 className={heading}>
          <button
            type="button"
            aria-expanded={open}
            aria-controls={`${id}-found`}
            onClick={() => onOpenChange?.(!open)}
            className="group flex w-full items-baseline gap-x-3 rounded-sm text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <span className="max-w-full shrink-0">{title}</span>
            <span className="min-w-0 flex-1 truncate text-[13px] font-normal tracking-normal text-muted-foreground">
              {!open && <span className="duration-500 animate-in fade-in motion-reduce:animate-none">{summary}</span>}
            </span>
            <ArrowDown01Icon
              aria-hidden="true"
              className={cn('h-4 w-4 shrink-0 self-center text-muted-foreground/60 transition-transform duration-300 group-hover:text-foreground motion-reduce:transition-none', open && 'rotate-180')}
            />
          </button>
        </h2>
      ) : (
        <h2 className={heading}>{title}</h2>
      )}
      {state === 'working' && log && log.length > 0 && (
        // The work as it happens; the step's title already tells screen readers what is going on.
        <ul aria-hidden="true" className="mt-2 space-y-1 font-mono text-[12.5px] leading-5 text-muted-foreground">
          {log.slice(-3).map((line, index, shown) => (
            <li key={`${log.length - shown.length + index}`} className="truncate duration-300 animate-in fade-in slide-in-from-bottom-1 motion-reduce:animate-none">
              <span className="text-muted-foreground/60">› </span>
              {line}
              {index === shown.length - 1 && (
                <span className="ml-1 inline-block h-3.5 w-[7px] translate-y-0.5 animate-[gapcursor-blink_1.1s_steps(2,start)_infinite] bg-muted-foreground/70 motion-reduce:animate-none" />
              )}
            </li>
          ))}
        </ul>
      )}
      {children && !foldable && (
        <div className="mt-3 duration-500 animate-in fade-in slide-in-from-bottom-2 motion-reduce:animate-none">{children}</div>
      )}
      {children && foldable && (
        <div
          id={`${id}-found`}
          inert={!open}
          // Typing in a result (the company name) keeps it open.
          onFocus={() => onOpenChange?.(true)}
          className={cn(
            'grid transition-[grid-template-rows,opacity] duration-300 ease-out motion-reduce:transition-none',
            open ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0',
          )}
        >
          {/* The side padding leaves room for focus rings inside the clipped box. */}
          <div className="-mx-1 min-h-0 overflow-hidden px-1">
            <div className="pb-1 pt-3 duration-500 animate-in fade-in slide-in-from-bottom-2 motion-reduce:animate-none">{children}</div>
          </div>
        </div>
      )}
    </li>
  );
}

/** A step's title; a number in it counts up from zero when the step finishes. */
function StepTitle({ step }: { step: StepView }) {
  const target = step.count;
  const animate = target !== undefined && target > 1 && !prefersReducedMotion();
  const [shown, setShown] = useState(animate ? 0 : target);
  useEffect(() => {
    if (!animate || target === undefined) return;
    let started: number | undefined;
    let frame = requestAnimationFrame(function tick(now) {
      started ??= now;
      const progress = Math.min(1, (now - started) / 800);
      // Fast at first, easing into the final number.
      setShown(Math.round(target * (1 - (1 - progress) ** 3)));
      if (progress < 1) frame = requestAnimationFrame(tick);
    });
    return () => cancelAnimationFrame(frame);
  }, [animate, target]);
  if (target === undefined || shown === undefined) return step.title;
  return <span className="tabular-nums">{step.title.replace(String(target), String(shown))}</span>;
}

const FACTS = [
  { title: 'No per-seat or per-resolution fees', text: 'Your bill doesn’t grow with each teammate you add or each question your agent answers.' },
  { title: 'Docs that keep up', text: 'Helpin groups the questions your agent couldn’t answer and can draft the missing article for your review.' },
  { title: 'From ticket to fix', text: 'Agents can take a ticket to a task, a pull request and a customer update, with the approvals your team chooses.' },
  { title: 'Open source', text: 'Use Helpin Cloud, or self-host the Community edition for free with your own AI models.' },
];

/** Something worth reading while the agent works: one fact about Helpin at a time. */
function WhileYouWait() {
  const [index, setIndex] = useState(0);
  useEffect(() => {
    const timer = setInterval(() => setIndex((current) => (current + 1) % FACTS.length), 5000);
    return () => clearInterval(timer);
  }, []);
  const fact = FACTS[index];
  return (
    <section aria-label="About Helpin" className="border-t pt-5">
      <p className="text-[11.5px] font-medium text-muted-foreground">While your agent works</p>
      <div key={index} className="mt-2 min-h-[4.25rem] duration-500 animate-in fade-in slide-in-from-bottom-1 motion-reduce:animate-none">
        <p className="text-sm font-medium leading-5">{fact.title}</p>
        <p className="mt-1 text-[13px] leading-[1.6] text-muted-foreground">{fact.text}</p>
      </div>
      <div aria-hidden="true" className="mt-3 flex gap-1.5">
        {FACTS.map((item, position) => (
          <span key={item.title} className={cn('h-0.5 w-6 rounded-full transition-colors duration-300', position === index ? 'bg-foreground' : 'bg-border')} />
        ))}
      </div>
    </section>
  );
}

function BrandSkeleton() {
  return (
    <div className="flex items-center gap-4" aria-hidden="true">
      <span className="h-12 w-12 shrink-0 animate-pulse rounded-[10px] bg-muted motion-reduce:animate-none" />
      <span className="flex-1 space-y-2">
        <span className="block h-4 w-40 animate-pulse rounded bg-muted motion-reduce:animate-none" />
        <span className="block h-3 w-24 animate-pulse rounded bg-muted motion-reduce:animate-none" />
      </span>
    </div>
  );
}

function BrandCard({ site, name, onNameChange }: { site: SupportQuickstartDetection; name: string; onNameChange: (name: string) => void }) {
  const id = useId();
  return (
    <div className="flex items-center gap-4">
      <BrandMark site={site} name={name} className="h-12 w-12 text-lg" />
      <div className="min-w-0 flex-1 space-y-1">
        <Label htmlFor={`${id}-name`} className="sr-only">Company name</Label>
        <Input
          id={`${id}-name`}
          value={name}
          onChange={(event) => onNameChange(event.target.value)}
          autoComplete="organization"
          maxLength={120}
          className="h-9 border-transparent px-1 text-base font-semibold shadow-none hover:border-input focus-visible:border-input"
        />
        <p className="flex items-center gap-2 truncate px-1 text-[12.5px] text-muted-foreground">
          {websiteDisplayName(site.website_url)}
          {site.brand_color && <span className="inline-block h-3 w-3 rounded-full border" style={{ background: site.brand_color }} title={`Brand color ${site.brand_color}`} />}
        </p>
      </div>
    </div>
  );
}

function HelpCenterFound({ host, found, onAdd }: {
  host: string;
  found: NonNullable<SupportPreviewRun['helpCenter']>;
  onAdd: (url: string) => void;
}) {
  if (!found.url) {
    return (
      <>
        <p className="text-[12.5px] leading-5 text-muted-foreground">Your agent will learn from {host}.</p>
        <AddHelpCenter onAdd={onAdd} />
      </>
    );
  }
  const label = siteLabel(found.url);
  if (!found.counts) {
    return <p className="text-[12.5px] leading-5 text-muted-foreground">{label} · your agent will learn from it</p>;
  }
  const collections = found.counts.top_collections.slice(0, 6);
  return (
    <div className="space-y-1 text-[12.5px] leading-5 text-muted-foreground">
      <p>{label} · {count(found.counts.collections, 'collection')} · copied into Helpin when you go live</p>
      {collections.length > 0 && (
        <p>
          {collections.map((collection, index) => (
            <span key={collection.name}>
              {index > 0 && ' · '}
              <span className="text-foreground/80">{collection.name}</span> {collection.articles}
            </span>
          ))}
        </p>
      )}
    </div>
  );
}

function AddHelpCenter({ onAdd }: { onAdd: (url: string) => void }) {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState('');
  if (!open) {
    return (
      <button type="button" className="mt-1 text-[12.5px] underline underline-offset-2" onClick={() => setOpen(true)}>
        Have one? Add its address
      </button>
    );
  }
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (value.trim()) onAdd(value.trim());
  };
  return (
    <form onSubmit={submit} className="mt-2 flex gap-2">
      <Input value={value} onChange={(event) => setValue(event.target.value)} placeholder="help.acme.com" aria-label="Help center address" inputMode="url" className="h-9" autoFocus />
      <Button type="submit" variant="outline" className="h-9">Use it</Button>
    </form>
  );
}

/** Account creation in place, as the last step of putting the agent live. */
function InlineSignup({ host, onSignedUp }: { host: string; onSignedUp: () => void }) {
  const id = useId();
  const signUp = useAuthStore((state) => state.signUp);
  const configuration = useAuthStore((state) => state.configuration);
  const [fullName, setFullName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [verificationEmail, setVerificationEmail] = useState<string | null>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    void ensureAuthConfiguration();
    headingRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
    headingRef.current?.focus();
  }, []);

  const signIn = <Link to={SIGN_IN_THEN_SETUP} className="underline underline-offset-2">Sign in</Link>;
  const domains = configuration?.signup_mode === 'domains' ? configuration.signup_allowed_domains ?? [] : [];

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setBusy(true);
    const result = await signUp(email, password, fullName);
    setBusy(false);
    if (result.error) {
      setError(result.error);
      return;
    }
    if (result.verificationRequired) {
      setVerificationEmail(result.email ?? email);
      return;
    }
    onSignedUp();
  };

  let body: ReactNode;
  if (verificationEmail) {
    body = <p className="text-[13px] leading-5 text-muted-foreground">We sent a link to {verificationEmail}. Open it and sign in; setup starts where you left off.</p>;
  } else if (configuration && !selfSignupAllowed(configuration)) {
    body = <p className="text-[13px] leading-5 text-muted-foreground">{INVITE_ONLY_MESSAGE} Already have an account? {signIn}</p>;
  } else {
    body = (
      <form onSubmit={(event) => void submit(event)} className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor={`${id}-name`}>Full name</Label>
            <Input id={`${id}-name`} autoComplete="name" value={fullName} onChange={(event) => setFullName(event.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor={`${id}-email`}>Work email</Label>
            <Input id={`${id}-email`} type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} required placeholder={`you@${host}`} />
          </div>
        </div>
        {domains.length > 0 && <p className="text-[12px] text-muted-foreground">Use your address at {formatDomainList(domains)}.</p>}
        <div className="space-y-2">
          <Label htmlFor={`${id}-password`}>Password</Label>
          <Input id={`${id}-password`} type="password" autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} required minLength={8} placeholder="At least 8 characters" />
        </div>
        <p role="alert" className="text-sm text-destructive empty:hidden">{error ?? ''}</p>
        <div className="flex flex-col-reverse gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-[12.5px] text-muted-foreground">Already have an account? {signIn}</p>
          <Button type="submit" size="lg" className="w-full sm:w-auto sm:min-w-56" disabled={busy}>
            {busy && <Loading01Icon className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />}
            {busy ? 'Creating your account…' : `Create account and go live`}
          </Button>
        </div>
      </form>
    );
  }
  return (
    <section aria-labelledby={`${id}-title`} className="space-y-4 rounded-xl border p-5">
      <div className="space-y-1">
        <h2 id={`${id}-title`} ref={headingRef} tabIndex={-1} className="text-base font-semibold outline-none">Last step: create your account</h2>
        <p className="text-[12.5px] text-muted-foreground">What we found is saved. Setup starts the moment you’re in.</p>
      </div>
      {body}
    </section>
  );
}
