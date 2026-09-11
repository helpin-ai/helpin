import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Tick01Icon } from '@/lib/icons';
import { playbookSetupSteps, type PlaybookSetupIssues } from '@/lib/crmPlaybookSetup';
import { cn } from '@/lib/utils';

export function PlaybookPublishAction({ issues, reason, busy, onPublish }: {
  issues: PlaybookSetupIssues; reason?: string; busy: boolean; onPublish: () => void;
}) {
  const [open, setOpen] = useState(false);
  const missing = playbookSetupSteps.filter((step) => issues[step.id].length > 0);
  const blocked = !!reason || missing.length > 0;
  return <Tooltip open={blocked && open} onOpenChange={setOpen}>
    <TooltipTrigger asChild>
      <Button type="button" size="sm" aria-label={busy ? 'Saving…' : 'Publish'} aria-disabled={blocked || busy}
        className={cn('gap-1.5 max-sm:size-8 max-sm:px-0', (blocked || busy) && 'opacity-50')}
        onPointerDown={(event) => { if (blocked) event.preventDefault(); }}
        onClick={(event) => {
          if (blocked) { event.preventDefault(); setOpen(true); return; }
          if (!busy) onPublish();
        }}>
        <Tick01Icon className="size-3.5" /><span className="max-sm:sr-only">{busy ? 'Saving…' : 'Publish'}</span>
      </Button>
    </TooltipTrigger>
    {blocked && <TooltipContent side="bottom" className="max-h-[70vh] max-w-sm overflow-y-auto">
      <div className="space-y-3">
        <p className="font-semibold">Complete setup to publish</p>
        {reason && <p>{reason}</p>}
        {missing.map((step) => <div key={step.id}><p className="font-semibold">{step.label}</p><ul className="mt-1 list-inside list-disc space-y-1">{issues[step.id].map((issue) => <li key={issue}>{issue}</li>)}</ul></div>)}
      </div>
    </TooltipContent>}
  </Tooltip>;
}
