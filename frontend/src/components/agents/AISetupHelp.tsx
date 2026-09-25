import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { InformationCircleIcon } from '@/lib/icons';

export function AISetupHelp({ label, description }: { label: string; description: string }) {
  return <QuickTooltip label={description}><button type="button" aria-label={label} className="inline-flex size-5 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"><InformationCircleIcon className="size-3.5" /></button></QuickTooltip>;
}
