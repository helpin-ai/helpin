import { Copy01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { InstallWithAIIcon } from './InstallWithAIIcon';

type WidgetInstallAIPromptProps = {
  prompt: string;
  onCopy: () => void;
};

export function WidgetInstallAIPrompt({ prompt, onCopy }: WidgetInstallAIPromptProps) {
  return (
    <div className="overflow-hidden rounded-lg border border-primary/20 bg-primary/[0.025]">
      <div className="flex flex-col gap-3 border-b border-border/70 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <InstallWithAIIcon className="h-[18px] w-[18px]" />
          </div>
          <p className="min-w-0 text-sm font-medium">Let your coding agent install Helpin</p>
        </div>
        <Button type="button" size="sm" className="shrink-0" onClick={onCopy}>
          <Copy01Icon className="mr-1.5 h-3.5 w-3.5" />
          Copy prompt
        </Button>
      </div>
      <Textarea
        aria-label="AI installation prompt"
        value={prompt}
        readOnly
        rows={16}
        className="max-h-[360px] resize-none rounded-none border-0 bg-muted/20 font-mono text-[11px] leading-relaxed shadow-none focus-visible:ring-0"
      />
      <div className="space-y-0.5 border-t border-border/70 px-4 py-2.5 text-xs text-muted-foreground">
        <p>Paste this prompt into Codex, Cursor, Claude Code, or another coding assistant.</p>
        <p className="text-muted-foreground/80">
          It includes your public widget key and setup checks. No private credentials are included.
        </p>
      </div>
    </div>
  );
}
