import { CheckCircle2, LoaderCircle, RefreshCw, Settings2, TriangleAlert } from 'lucide-react';

export type WizardStep = 'connect' | 'review';

export const STEP_ORDER: WizardStep[] = ['connect', 'review'];

export const STATUS_META: Record<string, { label: string; className: string; icon: typeof CheckCircle2 }> = {
  queued: { label: 'Pending', className: 'border-amber-500/40 bg-amber-500/10 text-amber-700', icon: LoaderCircle },
  running: { label: 'Syncing', className: 'border-sky-500/40 bg-sky-500/10 text-sky-700', icon: LoaderCircle },
  ready: { label: 'Ready', className: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700', icon: CheckCircle2 },
  failed: { label: 'Failed', className: 'border-destructive/40 bg-destructive/10 text-destructive', icon: TriangleAlert },
  stale: { label: 'Outdated', className: 'border-orange-500/40 bg-orange-500/10 text-orange-700', icon: RefreshCw },
  disabled: { label: 'Disabled', className: 'border-muted-foreground/30 bg-muted text-muted-foreground', icon: Settings2 },
};

export const CRAWL_SOURCE_OPTIONS = [
  { value: 'all', label: 'Sitemaps and links', description: 'Find pages using both sitemap files and in-page links.' },
  { value: 'sitemaps', label: 'Sitemaps only', description: 'Only import pages listed in the sitemap.' },
  { value: 'links', label: 'Links only', description: 'Discover pages by following links from the start URL.' },
] as const;
