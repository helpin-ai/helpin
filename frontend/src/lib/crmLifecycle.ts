import type { LifecycleStage } from './crmTypes';

/** CRM lifecycle stages in funnel order, with display labels and compact labels for tight menus. */
export const LIFECYCLE_STAGE_OPTIONS: { value: LifecycleStage; label: string; shortLabel?: string }[] = [
  { value: 'subscriber', label: 'Subscriber' },
  { value: 'lead', label: 'Lead' },
  { value: 'marketing_qualified', label: 'Marketing Qualified', shortLabel: 'MQL' },
  { value: 'sales_qualified', label: 'Sales Qualified', shortLabel: 'SQL' },
  { value: 'opportunity', label: 'Opportunity' },
  { value: 'customer', label: 'Customer' },
  { value: 'evangelist', label: 'Evangelist' },
];

/** Badge colors shared with the CRM contacts table. */
export const LIFECYCLE_BADGE_CLASSNAMES: Record<LifecycleStage, string> = {
  subscriber: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  lead: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
  marketing_qualified: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
  sales_qualified: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300',
  opportunity: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
  customer: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
  evangelist: 'bg-pink-100 text-pink-700 dark:bg-pink-900 dark:text-pink-300',
};

/** Dot colors for lifecycle options in menus. */
export const LIFECYCLE_DOT_CLASSNAMES: Record<LifecycleStage, string> = {
  subscriber: 'bg-gray-500',
  lead: 'bg-blue-500',
  marketing_qualified: 'bg-purple-500',
  sales_qualified: 'bg-indigo-500',
  opportunity: 'bg-orange-500',
  customer: 'bg-green-500',
  evangelist: 'bg-pink-500',
};

/** Returns the display label for a lifecycle stage, falling back to the raw value. */
export function lifecycleStageLabel(stage: string): string {
  return LIFECYCLE_STAGE_OPTIONS.find((option) => option.value === stage)?.label ?? stage;
}
