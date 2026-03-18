import ReactMarkdown from 'react-markdown';
import { FileText } from 'lucide-react';

import type { PlanningSession } from '@/lib/pmTypes';

interface Props {
  session?: PlanningSession;
}

export function PlanningSpecPreview({ session }: Props) {
  const draft = session?.spec_draft || '';

  if (!draft) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-8 text-center">
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
          <FileText className="h-6 w-6 text-muted-foreground" />
        </div>
        <div>
          <p className="text-sm font-medium text-muted-foreground">Spec Preview</p>
          <p className="mt-1 text-xs text-muted-foreground/70">
            The product specification will appear here as the planner drafts it during the conversation.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto p-4">
      <div className="mb-3 flex items-center gap-2">
        <FileText className="h-4 w-4 text-muted-foreground" />
        <h3 className="text-xs font-semibold">Product Specification</h3>
      </div>
      <div className="prose prose-sm dark:prose-invert max-w-none text-xs [&_h1]:text-base [&_h2]:text-sm [&_h3]:text-xs [&_h4]:text-xs [&_p]:text-xs [&_li]:text-xs [&_code]:text-[11px] [&_pre]:text-[11px]">
        <ReactMarkdown>{draft}</ReactMarkdown>
      </div>
    </div>
  );
}
