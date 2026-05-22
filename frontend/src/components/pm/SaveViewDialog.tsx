import { useEffect, useState } from 'react';
import { GlobeIcon, LockIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

export function SaveViewDialog({
  open,
  onOpenChange,
  onSave,
  initialName,
  initialIsShared,
  title,
  saveLabel = 'Save view',
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSave: (name: string, isShared: boolean) => void;
  initialName?: string;
  initialIsShared?: boolean;
  title: string;
  saveLabel?: string;
}) {
  const [name, setName] = useState(initialName ?? '');
  const [isShared, setIsShared] = useState(initialIsShared ?? false);

  useEffect(() => {
    if (open) {
      setName(initialName ?? '');
      setIsShared(initialIsShared ?? false);
    }
  }, [open, initialName, initialIsShared]);

  const submit = () => {
    if (!name.trim()) return;
    onSave(name.trim(), isShared);
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[400px] gap-0 p-0">
        <DialogHeader className="px-5 pt-5 pb-5">
          <DialogTitle className="text-sm font-medium">{title}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 px-5 pb-5">
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Untitled view"
            autoFocus
            className="h-9"
            onKeyDown={(e) => {
              if (e.key === 'Enter') submit();
            }}
          />
          <div className="grid grid-cols-2 gap-1 rounded-md border border-border/70 bg-muted/30 p-0.5">
            <button
              type="button"
              onClick={() => setIsShared(false)}
              className={`flex items-center justify-center gap-1.5 rounded px-2 py-1.5 text-xs font-medium transition-colors ${
                !isShared
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              <LockIcon className="h-3 w-3" />
              Personal
            </button>
            <button
              type="button"
              onClick={() => setIsShared(true)}
              className={`flex items-center justify-center gap-1.5 rounded px-2 py-1.5 text-xs font-medium transition-colors ${
                isShared
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              <GlobeIcon className="h-3 w-3" />
              Shared
            </button>
          </div>
          <p className="text-[11px] text-muted-foreground">
            {isShared ? 'Visible to everyone in this workspace.' : 'Only visible to you.'}
          </p>
        </div>
        <DialogFooter className="border-t border-border/60 px-5 py-2.5">
          <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button size="sm" disabled={!name.trim()} onClick={submit}>
            {saveLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
