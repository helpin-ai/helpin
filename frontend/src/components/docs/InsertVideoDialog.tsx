import { useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AlertCircle } from 'lucide-react';
import { parseVideoUrl, SUPPORTED_PROVIDERS, type VideoInfo } from './videoProviders';

interface InsertVideoDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onInsert: (info: VideoInfo) => void;
}

export function InsertVideoDialog({ open, onOpenChange, onInsert }: InsertVideoDialogProps) {
  const [url, setUrl] = useState('');
  const parsed = parseVideoUrl(url);
  const showError = url.trim().length > 0 && !parsed;

  const handleInsert = () => {
    if (!parsed) return;
    onInsert(parsed);
    setUrl('');
    onOpenChange(false);
  };

  const handleOpenChange = (open: boolean) => {
    if (!open) setUrl('');
    onOpenChange(open);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Insert video</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 py-2">
          <div className="space-y-1.5">
            <Label>Video URL</Label>
            <p className="text-sm text-muted-foreground">
              Copy and paste your video's URL from {SUPPORTED_PROVIDERS}
            </p>
          </div>
          <Input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && parsed) handleInsert(); }}
            placeholder="www.youtube.com"
            autoFocus
          />
          {showError && (
            <p className="flex items-center gap-1 text-xs text-destructive">
              <AlertCircle className="h-3 w-3 shrink-0" />
              Unsupported URL. Use {SUPPORTED_PROVIDERS}.
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleInsert} disabled={!parsed}>
            Insert
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
