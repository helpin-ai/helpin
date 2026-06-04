import { useCallback } from 'react';
import { useNavigate } from '@tanstack/react-router';
import type { JSONContent } from '@tiptap/react';
import { ArrowUpRight01Icon, Loading01Icon } from '@/lib/icons';
import { useDocsContent, useDocsDocument } from '@/hooks/queries';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { DocsEditor } from '@/components/docs/DocsEditor';

interface DocumentPreviewDialogProps {
  workspaceId: string;
  slug: string;
  docId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function DocumentPreviewDialog({
  workspaceId,
  slug,
  docId,
  open,
  onOpenChange,
}: DocumentPreviewDialogProps) {
  const navigate = useNavigate();
  const effectiveDocId = open ? (docId ?? '') : '';
  const { data: doc, isLoading: docLoading, error: docError } = useDocsDocument(workspaceId, effectiveDocId);
  const { data: content, isLoading: contentLoading, error: contentError } = useDocsContent(workspaceId, effectiveDocId);

  const handleOpenInDocs = useCallback(() => {
    if (!docId || !slug) return;
    onOpenChange(false);
    navigate({ to: '/w/$slug/docs/documents/$docId', params: { slug, docId } } as any);
  }, [docId, navigate, onOpenChange, slug]);

  const isLoading = docLoading || contentLoading;
  const errorMessage =
    (docError instanceof Error ? docError.message : null)
    ?? (contentError instanceof Error ? contentError.message : null);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[90vh] max-h-[90vh] min-h-0 flex-col gap-0 overflow-hidden p-0 sm:max-w-5xl">
        <DialogHeader className="shrink-0 border-b border-border/60 px-6 py-4 pr-16">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 flex-1 space-y-1">
              <DialogTitle className="truncate text-base" title={doc?.title || undefined}>
                {doc?.title || (isLoading ? 'Loading document...' : 'Document preview')}
              </DialogTitle>
              {doc?.excerpt ? (
                <DialogDescription className="line-clamp-2">
                  {doc.excerpt}
                </DialogDescription>
              ) : null}
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="shrink-0 gap-1.5"
              onClick={handleOpenInDocs}
              disabled={!docId || !slug}
            >
              Open in Docs
              <ArrowUpRight01Icon className="h-3.5 w-3.5" />
            </Button>
          </div>
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-hidden bg-background">
          {isLoading ? (
            <div className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground">
              <Loading01Icon className="h-4 w-4 animate-spin" />
              Loading document...
            </div>
          ) : errorMessage ? (
            <div className="flex h-full items-center justify-center px-6 text-center text-sm text-muted-foreground">
              {errorMessage}
            </div>
          ) : (
            <div className="flex h-full min-h-0 flex-col overflow-hidden">
              <DocsEditor
                key={`${doc?.id ?? effectiveDocId}:${content?.updated_at ?? 'empty'}`}
                initialContent={(content?.content as JSONContent | null | undefined) ?? null}
                onSave={async () => {}}
                readOnly
              />
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
