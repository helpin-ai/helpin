import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useCreateOrganization, useOrganizations } from '@/hooks/queries';
import { Alert01Icon, BookOpen01Icon, BubbleChatIcon, CheckmarkCircle02Icon, GlobeIcon, Loading01Icon } from '@/lib/icons';
import { supportQuickstartService, type QuickstartAIMode, type SupportQuickstartDetection } from '@/lib/services/supportQuickstartService';
import { workspacesService } from '@/lib/services/workspacesService';
import { generateWorkspaceSlug } from '@/lib/slugUtils';
import type { Workspace } from '@/lib/types';
import { siteLabel, websiteDisplayName } from '@/lib/workspaceOnboardingFlow';
import { shouldShowWorkspaceOnboardingOrganizationSelector } from '@/lib/workspaceOnboardingMode';
import { cn } from '@/lib/utils';
import { useAuthStore } from '@/stores/authStore';
import { useOrganizationStore } from '@/stores/organizationStore';
import { OnboardingActions, OnboardingTextButton } from './OnboardingShell';

/** Asks for the company website. */
export function WebsiteForm({ defaultWebsite, onSubmit }: {
  defaultWebsite: string;
  onSubmit: (website: string) => void;
}) {
  const id = useId();
  const [website, setWebsite] = useState(defaultWebsite);
  const [error, setError] = useState<string | null>(null);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!website.trim()) {
      setError('Enter your website, such as acme.com.');
      return;
    }
    setError(null);
    onSubmit(website.trim());
  };

  return (
    <form onSubmit={submit} noValidate className="space-y-7">
      <div className="space-y-2">
        <Label htmlFor={`${id}-website`}>Company website</Label>
        <Input
          id={`${id}-website`}
          value={website}
          onChange={(event) => setWebsite(event.target.value)}
          placeholder="acme.com"
          inputMode="url"
          autoComplete="url"
          autoFocus
          aria-invalid={Boolean(error)}
          aria-describedby={`${id}-hint ${id}-error`}
          className="h-12 text-base"
        />
        <p id={`${id}-hint`} className="text-[12.5px] text-muted-foreground">We’ll read your site and help center. Nothing is changed on them, and no account is needed to look.</p>
      </div>
      <p id={`${id}-error`} role="alert" className="text-sm text-destructive empty:hidden">{error ?? ''}</p>
      <OnboardingActions>
        <Button type="submit" size="lg" className="w-full sm:w-auto sm:min-w-44">Show me</Button>
      </OnboardingActions>
    </form>
  );
}

const AI_MODES: { value: QuickstartAIMode; title: string; description: string }[] = [
  { value: 'answer', title: 'Answer customers', description: 'The AI replies when your docs cover the question and hands the rest to your team.' },
  { value: 'draft', title: 'Draft replies for my team', description: 'The AI writes a private suggested reply. A teammate reviews and sends it.' },
];

/**
 * Shows what was found on the website, takes the company name and AI mode,
 * then creates the workspace and starts the support setup. With `autoStart`
 * it starts with the `initial` choices made before signup, and shows the
 * form only if that fails.
 */
export function SetupReview({ detection, initial, autoStart = false, onBack, onStarted }: {
  detection: SupportQuickstartDetection;
  initial?: { companyName?: string; helpCenterUrl?: string; aiMode?: QuickstartAIMode };
  autoStart?: boolean;
  onBack: () => void;
  onStarted: (workspace: Workspace) => void;
}) {
  const id = useId();
  const queryClient = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const { data: organizations = [], isLoading: organizationsLoading } = useOrganizations();
  const createOrganization = useCreateOrganization();
  const { currentOrganization, setCurrentOrganization } = useOrganizationStore();
  const [organizationChoice, setOrganizationChoice] = useState<string | null>(null);
  const [name, setName] = useState(initial?.companyName ?? detection.company_name);
  const [helpCenterUrl, setHelpCenterUrl] = useState(initial?.helpCenterUrl ?? detection.help_center?.url ?? '');
  const [addingHelpCenter, setAddingHelpCenter] = useState(false);
  const [mode, setMode] = useState<QuickstartAIMode>(initial?.aiMode ?? 'answer');
  const [created, setCreated] = useState<Workspace | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const autoOrganization = useRef(false);
  const autoStarted = useRef(false);

  // Signup creates an organization; this covers accounts where it didn't.
  useEffect(() => {
    if (organizationsLoading || organizations.length > 0 || autoOrganization.current || !user) return;
    autoOrganization.current = true;
    const orgName = `${user.full_name?.split(' ')[0] || 'My'}'s Organization`;
    createOrganization.mutateAsync({ name: orgName, slug: generateWorkspaceSlug(orgName) })
      .then((organization) => setCurrentOrganization(organization))
      .catch(() => setError('Your account isn’t ready yet. Reload the page and try again.'));
  }, [createOrganization, organizations.length, organizationsLoading, setCurrentOrganization, user]);

  const organization = organizations.find((item) => item.id === organizationChoice)
    ?? currentOrganization ?? organizations[0] ?? null;
  const websiteHost = websiteDisplayName(detection.website_url);
  const helpCenter = detection.help_center;
  const helpCenterHost = helpCenter ? siteLabel(helpCenter.url) : '';

  const run = async () => {
    const companyName = name.trim();
    if (!companyName) {
      setError('Enter your company name.');
      return;
    }
    if (!organization && !created) {
      setError('Your account isn’t ready yet. Wait a moment and try again.');
      return;
    }
    setError(null);
    setBusy(true);
    try {
      let workspace = created;
      if (!workspace && organization) {
        setCurrentOrganization(organization);
        const result = await workspacesService.create({
          name: companyName,
          slug: generateWorkspaceSlug(companyName),
          workspace_key: (companyName.replace(/[^a-zA-Z]/g, '').slice(0, 3) || 'WS').toUpperCase(),
          organization_id: organization.id,
          timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          website_url: detection.website_url,
          setup_goals: ['customer_support'],
        });
        if (result.error || !result.data) {
          setError(result.error ?? 'The workspace couldn’t be created. Try again.');
          return;
        }
        workspace = result.data;
        // Kept so a retry starts setup without creating a second workspace.
        setCreated(workspace);
        void queryClient.invalidateQueries({ queryKey: ['workspaces'] });
      }
      if (!workspace) return;
      const { error: startError } = await supportQuickstartService.start(workspace.id, {
        website_url: detection.website_url,
        company_name: companyName,
        help_center_url: helpCenterUrl.trim() || undefined,
        ai_mode: mode,
      });
      if (startError) {
        setError(startError);
        return;
      }
      onStarted(workspace);
    } finally {
      setBusy(false);
    }
  };

  // Starts once the organization is known; the form takes over if it fails.
  useEffect(() => {
    if (!autoStart || autoStarted.current || organizationsLoading || !organization) return;
    autoStarted.current = true;
    void run();
  });

  if (autoStart && !error) {
    return (
      <div role="status" className="space-y-2">
        <p className="flex items-center gap-2 text-sm font-medium">
          <Loading01Icon className="h-4 w-4 animate-spin text-muted-foreground" aria-hidden="true" />
          Setting up {name.trim() || websiteHost}…
        </p>
        <p className="text-[13px] leading-[1.6] text-muted-foreground">
          Creating your workspace, starting on your help center and website, and putting your brand on the chat widget. This takes about 20 seconds.
        </p>
      </div>
    );
  }

  const submit = (event: FormEvent) => {
    event.preventDefault();
    void run();
  };

  // The square holds an icon; a wordmark logo only when there is no icon.
  const mark = detection.favicon_url || detection.logo_url;
  return (
    <form onSubmit={submit} noValidate className="space-y-8">
      <div className="flex items-center gap-4 rounded-xl border p-4">
        <span
          aria-hidden="true"
          className={cn('flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-lg border text-lg font-semibold', mark ? 'bg-white' : 'bg-foreground text-background')}
          style={!mark && detection.brand_color ? { background: detection.brand_color, color: '#fff' } : undefined}
        >
          {mark ? <img src={mark} alt="" className="max-h-9 max-w-9 object-contain" /> : (name.trim()[0] ?? '?').toUpperCase()}
        </span>
        <div className="min-w-0 flex-1 space-y-1">
          <Label htmlFor={`${id}-name`} className="sr-only">Company name</Label>
          <Input
            id={`${id}-name`}
            value={name}
            onChange={(event) => setName(event.target.value)}
            autoComplete="organization"
            className="h-9 border-transparent px-1 text-base font-semibold shadow-none hover:border-input focus-visible:border-input"
          />
          <p className="truncate px-1 text-[12.5px] text-muted-foreground">{websiteHost}</p>
        </div>
      </div>

      <section aria-labelledby={`${id}-plan`} className="space-y-3">
        <h2 id={`${id}-plan`} className="text-sm font-medium">What we’ll set up</h2>
        {!detection.website_readable && (
          <p className="flex gap-2 rounded-lg bg-amber-50 p-3 text-[13px] leading-5 text-amber-900 dark:bg-amber-950 dark:text-amber-200">
            <Alert01Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span>We couldn’t read {websiteHost}. You can continue, or <button type="button" className="underline underline-offset-2" onClick={onBack}>change the address</button>.</span>
          </p>
        )}
        <ul className="divide-y rounded-xl border">
          <PlanItem icon={BookOpen01Icon} title={helpCenter?.platform === 'intercom' ? `Copy your help center from ${helpCenterHost}` : helpCenter ? `Learn from your help center at ${helpCenterHost}` : 'Help articles'}>
            {helpCenter?.platform === 'intercom'
              ? 'Every article, collection and image, published in Helpin.'
              : helpCenter
                ? 'Your AI agent reads your articles to answer questions.'
                : addingHelpCenter
                  ? <Input value={helpCenterUrl} onChange={(event) => setHelpCenterUrl(event.target.value)} placeholder="help.acme.com" aria-label="Help center address" className="mt-2 h-9" autoFocus />
                  : <>We didn’t find a help center. <button type="button" className="underline underline-offset-2" onClick={() => setAddingHelpCenter(true)}>Add its address</button> or write articles later.</>}
          </PlanItem>
          <PlanItem icon={GlobeIcon} title={`Learn from ${websiteHost}`}>Product and pricing pages, so the AI can answer beyond your articles.</PlanItem>
          <PlanItem icon={BubbleChatIcon} title="Inbox and chat widget in your brand">Conversations from chat and email arrive in one shared inbox.</PlanItem>
        </ul>
      </section>

      <AIModeChoice value={mode} onChange={setMode} />

      {!created && shouldShowWorkspaceOnboardingOrganizationSelector({ organizationCount: organizations.length }) && (
        <div className="space-y-2">
          <Label htmlFor={`${id}-organization`}>Organization</Label>
          <Select value={organization?.id ?? ''} onValueChange={setOrganizationChoice}>
            <SelectTrigger id={`${id}-organization`} variant="underline" className="w-full px-0.5"><SelectValue placeholder="Select organization" /></SelectTrigger>
            <SelectContent>
              {organizations.map((item) => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      )}

      <p role="alert" className="text-sm text-destructive empty:hidden">{error ?? ''}</p>
      <OnboardingActions className="sm:justify-between">
        {!created ? <OnboardingTextButton onClick={onBack} disabled={busy}>Back</OnboardingTextButton> : <span />}
        <Button type="submit" size="lg" className="w-full sm:w-auto sm:min-w-44" disabled={busy || organizationsLoading}>
          {busy && <Loading01Icon className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />}
          {busy ? 'Setting up…' : created ? 'Try again' : 'Set up support'}
        </Button>
      </OnboardingActions>
    </form>
  );
}

/** How the AI helps: answer customers itself, or draft replies for the team. */
function AIModeChoice({ value, onChange }: { value: QuickstartAIMode; onChange: (mode: QuickstartAIMode) => void }) {
  const id = useId();
  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-medium">How should the AI help?</legend>
      <div className="grid gap-3 sm:grid-cols-2">
        {AI_MODES.map((option) => (
          <label
            key={option.value}
            className={cn(
              'flex cursor-pointer flex-col rounded-xl border p-4 transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring',
              value === option.value ? 'border-foreground/70 bg-muted/50' : 'hover:border-foreground/25 hover:bg-muted/30',
            )}
          >
            <span className="flex items-center gap-2">
              <input type="radio" name={`${id}-mode`} value={option.value} checked={value === option.value} onChange={() => onChange(option.value)} className="sr-only" />
              <span aria-hidden="true" className={cn('flex h-4 w-4 items-center justify-center rounded-full border', value === option.value ? 'border-foreground' : 'border-input')}>
                {value === option.value && <span className="h-2 w-2 rounded-full bg-foreground" />}
              </span>
              <span className="text-[13.5px] font-semibold">{option.title}</span>
            </span>
            <span className="mt-2 text-[12.5px] leading-5 text-muted-foreground">{option.description}</span>
          </label>
        ))}
      </div>
      <p className="text-[12px] text-muted-foreground">You can change this any time in settings.</p>
    </fieldset>
  );
}

function PlanItem({ icon: Icon, title, children }: { icon: typeof CheckmarkCircle02Icon; title: string; children: ReactNode }) {
  return (
    <li className="flex gap-3 p-4">
      <Icon className="mt-0.5 h-[18px] w-[18px] shrink-0 text-muted-foreground" aria-hidden="true" />
      <div className="min-w-0 flex-1">
        <p className="text-[13.5px] font-medium leading-5">{title}</p>
        <div className="mt-0.5 text-[12.5px] leading-5 text-muted-foreground">{children}</div>
      </div>
    </li>
  );
}
