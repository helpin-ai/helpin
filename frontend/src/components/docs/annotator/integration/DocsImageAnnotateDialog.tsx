import { useCallback, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Loading01Icon } from '@/lib/icons';
import { uploadEditorImage, type EditorUploadConfig } from '@/hooks/useEditorImageUpload';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import {
  DEFAULT_ANNOTATION_COLOR,
  DEFAULT_FONT_SIZE,
  DEFAULT_STROKE_WIDTH,
  hasCoverShape,
  type AnnotationShape,
  type AnnotationState,
} from '../core/annotationTypes';
import { ImageAnnotator, type AnnotationTool } from '../core/ImageAnnotator';
import { renderAnnotationsToFile } from '../core/renderAnnotations';
import { useAnnotationHistory } from '../core/useAnnotationHistory';
import { AnnotatorToolbar } from './AnnotatorToolbar';

export interface AnnotationSaveResult {
  attachmentId: string;
  url: string;
  /** Null when the user chose to flatten permanently — the annotation is then not re-editable. */
  annotationState: AnnotationState | null;
  sourceAttachmentId: string | null;
}

interface DocsImageAnnotateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  uploadConfig: EditorUploadConfig;
  /** The image the annotations are drawn on — always the ORIGINAL, never a previous render. */
  sourceUrl: string;
  sourceAttachmentId: string | null;
  initialState: AnnotationState | null;
  fileName?: string;
  onSave: (result: AnnotationSaveResult) => void;
}

export function DocsImageAnnotateDialog({
  open,
  onOpenChange,
  workspaceId,
  uploadConfig,
  sourceUrl,
  sourceAttachmentId,
  initialState,
  fileName = 'image',
  onSave,
}: DocsImageAnnotateDialogProps) {
  // The caller mounts this only while open, so state initialisers are the reset — no effect
  // needs to sync `open` back into local state.
  const history = useAnnotationHistory(initialState?.shapes ?? []);
  const [tool, setTool] = useState<AnnotationTool>('select');
  const [color, setColor] = useState(DEFAULT_ANNOTATION_COLOR);
  const [strokeWidth, setStrokeWidth] = useState(DEFAULT_STROKE_WIDTH);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [baseSize, setBaseSize] = useState<{ width: number; height: number } | null>(
    initialState ? { width: initialState.baseWidth, height: initialState.baseHeight } : null,
  );
  const [saving, setSaving] = useState(false);
  const [flattenRequested, setFlattenRequested] = useState(() => hasCoverShape(initialState));

  const containsCover = useMemo(
    () => history.shapes.some((shape: AnnotationShape) => shape.type === 'cover'),
    [history.shapes],
  );

  // Redaction forces permanent flattening: a cover box that leaves the original fetchable is not
  // a redaction at all. Derived rather than stored, so adding a cover shape cannot race the save.
  const flattenPermanently = flattenRequested || containsCover;

  const handleImageLoaded = useCallback((size: { width: number; height: number }) => {
    setBaseSize(size);
  }, []);

  const handleError = useCallback((message: string) => {
    toast.error(message);
  }, []);

  const deleteSelected = useCallback(() => {
    if (!selectedId) return;
    history.commit(history.shapes.filter((shape) => shape.id !== selectedId));
    setSelectedId(null);
  }, [history, selectedId]);

  const save = useCallback(async () => {
    if (!baseSize) return;
    setSaving(true);
    try {
      const state: AnnotationState = {
        version: 1,
        baseWidth: baseSize.width,
        baseHeight: baseSize.height,
        shapes: history.shapes,
      };

      const safeName = fileName.replace(/\.[a-z0-9]+$/i, '') || 'image';
      const file = await renderAnnotationsToFile(sourceUrl, state, `${safeName}-annotated.png`);
      const upload = await uploadEditorImage(file, uploadConfig);

      const permanent = flattenPermanently;
      if (permanent && sourceAttachmentId) {
        // Deleting the original is the whole point of a permanent flatten. If it fails we must
        // not apply the annotation: doing so would show a redacted image while the un-redacted
        // original stayed fetchable, which reads as "redacted" and is not.
        const { error } = await pmAttachmentService.remove(workspaceId, sourceAttachmentId);
        if (error) {
          throw new Error(`The original image could not be deleted, so the redaction was not applied: ${error}`);
        }
      }

      onSave({
        attachmentId: upload.attachmentId,
        url: upload.publicUrl,
        annotationState: permanent ? null : state,
        sourceAttachmentId: permanent ? null : sourceAttachmentId,
      });
      onOpenChange(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not save the annotated image');
    } finally {
      setSaving(false);
    }
  }, [
    baseSize,
    fileName,
    flattenPermanently,
    history.shapes,
    onOpenChange,
    onSave,
    sourceAttachmentId,
    sourceUrl,
    uploadConfig,
    workspaceId,
  ]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[90vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-5xl">
        <DialogHeader className="border-b px-5 py-4 text-left sm:text-left">
          <DialogTitle>Annotate image</DialogTitle>
          <DialogDescription className="mt-1 max-w-2xl">
            Add arrows, boxes, notes, and redactions. The original image is kept so you can edit these
            annotations later.
          </DialogDescription>
        </DialogHeader>

        <div className="border-b px-5 py-3">
          <AnnotatorToolbar
            tool={tool}
            onToolChange={setTool}
            color={color}
            onColorChange={setColor}
            strokeWidth={strokeWidth}
            onStrokeWidthChange={setStrokeWidth}
            canUndo={history.canUndo}
            canRedo={history.canRedo}
            onUndo={history.undo}
            onRedo={history.redo}
            canDelete={Boolean(selectedId)}
            onDelete={deleteSelected}
          />
        </div>

        <div className="min-h-0 flex-1 overflow-auto bg-muted/30 p-5">
          <ImageAnnotator
            imageUrl={sourceUrl}
            tool={tool}
            color={color}
            strokeWidth={strokeWidth}
            fontSize={DEFAULT_FONT_SIZE}
            shapes={history.shapes}
            onCommit={history.commit}
            selectedId={selectedId}
            onSelectedIdChange={setSelectedId}
            onImageLoaded={handleImageLoaded}
            onError={handleError}
            onToolConsumed={() => setTool('select')}
          />
        </div>

        <div className="border-t px-5 py-4">
          {containsCover && (
            <div className="mb-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-200">
              This annotation redacts part of the image. The original will be deleted on save so the
              hidden content cannot be recovered — the annotation will not be editable afterwards.
            </div>
          )}
          <div className="flex items-center gap-2 pb-3">
            <Checkbox
              id="annotation-flatten"
              checked={flattenPermanently}
              disabled={containsCover}
              onCheckedChange={(checked) => setFlattenRequested(checked === true)}
            />
            <label htmlFor="annotation-flatten" className="text-xs text-muted-foreground">
              Flatten permanently (delete the original and discard editable annotations)
            </label>
          </div>
          <DialogFooter className="sm:justify-between">
            <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={saving}>
              Cancel
            </Button>
            <Button onClick={() => void save()} disabled={saving || !baseSize}>
              {saving && <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />}
              Save annotated image
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  );
}
