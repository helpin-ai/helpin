import { Button } from '@/components/ui/button';

interface DetailDescriptionEditorActionsProps {
  onCancel: () => void;
  onDone: () => void;
}

export function DetailDescriptionEditorActions({
  onCancel,
  onDone,
}: DetailDescriptionEditorActionsProps) {
  return (
    <div className="sticky bottom-0 z-10 mt-2 flex items-center justify-end gap-2 border-t border-border/60 bg-background/95 py-2 backdrop-blur-sm transition-colors group-focus-within/description-editor:border-foreground/70">
      <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
        Cancel
      </Button>
      <Button type="button" variant="default" size="sm" onClick={onDone}>
        Done
      </Button>
    </div>
  );
}
