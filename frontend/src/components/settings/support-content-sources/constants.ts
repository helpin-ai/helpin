import { CheckmarkCircle02Icon, Loading03Icon, ArrowReloadHorizontalIcon, Settings02Icon, Alert01Icon } from '@/lib/icons';

export type WizardStep = 'connect' | 'review';

export const STEP_ORDER: WizardStep[] = ['connect', 'review'];

export const STATUS_META: Record<string, { label: string; className: string; icon: typeof CheckmarkCircle02Icon }> = {
  queued: { label: 'Pending', className: 'border-amber-500/40 bg-amber-500/10 text-amber-700', icon: Loading03Icon },
  running: { label: 'Syncing', className: 'border-sky-500/40 bg-sky-500/10 text-sky-700', icon: Loading03Icon },
  ready: { label: 'Ready', className: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700', icon: CheckmarkCircle02Icon },
  failed: { label: 'Failed', className: 'border-destructive/40 bg-destructive/10 text-destructive', icon: Alert01Icon },
  stale: { label: 'Outdated', className: 'border-orange-500/40 bg-orange-500/10 text-orange-700', icon: ArrowReloadHorizontalIcon },
  disabled: { label: 'Disabled', className: 'border-muted-foreground/30 bg-muted text-muted-foreground', icon: Settings02Icon },
};

export const CRAWL_SOURCE_OPTIONS = [
  { value: 'all', label: 'Sitemaps and links', description: 'Find pages using both sitemap files and in-page links.' },
  { value: 'sitemaps', label: 'Sitemaps only', description: 'Only import pages listed in the sitemap.' },
  { value: 'links', label: 'Links only', description: 'Discover pages by following links from the start URL.' },
] as const;
