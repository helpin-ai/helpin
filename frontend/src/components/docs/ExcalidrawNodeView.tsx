import { useCallback, useEffect, useMemo, useRef, useState, type ComponentType } from 'react';
import { NodeViewWrapper, type JSONContent, type NodeViewProps } from '@tiptap/react';
import { useTheme } from 'next-themes';
import '@excalidraw/excalidraw/index.css';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Download04Icon, PencilEdit01Icon, SourceCodeIcon, Delete01Icon } from '@/lib/icons';
import {
  excalidrawPngFile,
  exportExcalidrawPngBlob,
  isExcalidrawSceneEmpty,
  normalizeExcalidrawScene,
  type ExcalidrawScene,
  type ExcalidrawExportTheme,
} from '@/lib/excalidrawRenderer';

type ExcalidrawComponent = ComponentType<Record<string, unknown>>;
type ImmediateSaveHandler = (content: JSONContent) => void | Promise<void>;

function titleFromAttrs(value: unknown): string {
  const title = typeof value === 'string' ? value.trim() : '';
  return title || 'Excalidraw drawing';
}

export function ExcalidrawNodeView({ node, updateAttributes, deleteNode, editor }: NodeViewProps) {
  const editable = editor.isEditable;
  const { resolvedTheme } = useTheme();
  const excalidrawTheme: ExcalidrawExportTheme = resolvedTheme === 'dark' ? 'dark' : 'light';
  const title = titleFromAttrs(node.attrs.title);
  const scene = useMemo(() => normalizeExcalidrawScene(node.attrs.scene), [node.attrs.scene]);
  const themedScene = useMemo<ExcalidrawScene>(() => ({
    ...scene,
    appState: {
      ...scene.appState,
      theme: excalidrawTheme,
      viewBackgroundColor: 'transparent',
    },
  }), [excalidrawTheme, scene]);
  const isEmpty = isExcalidrawSceneEmpty(scene);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editorInitialScene, setEditorInitialScene] = useState<ExcalidrawScene>(themedScene);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [Excalidraw, setExcalidraw] = useState<ExcalidrawComponent | null>(null);
  const [savingDraft, setSavingDraft] = useState(false);
  const sceneRef = useRef(scene);
  const draftSceneRef = useRef(scene);
  const immediateSave = useMemo(() => {
    const extension = editor.extensionManager.extensions.find((item) => item.name === 'excalidraw');
    return (extension?.options as { onImmediateSave?: ImmediateSaveHandler } | undefined)?.onImmediateSave;
  }, [editor]);

  useEffect(() => {
    sceneRef.current = scene;
    if (!dialogOpen) {
      draftSceneRef.current = scene;
      setEditorInitialScene(themedScene);
    }
  }, [dialogOpen, scene, themedScene]);

  useEffect(() => {
    let cancelled = false;
    let url: string | null = null;

    if (isEmpty) {
      setPreviewUrl(null);
      setPreviewError(null);
      return undefined;
    }

    exportExcalidrawPngBlob(scene, excalidrawTheme)
      .then((blob) => {
        if (cancelled) return;
        url = URL.createObjectURL(blob);
        setPreviewUrl(url);
        setPreviewError(null);
      })
      .catch((error) => {
        if (cancelled) return;
        setPreviewUrl(null);
        setPreviewError(error instanceof Error ? error.message : 'Unable to render drawing');
      });

    return () => {
      cancelled = true;
      if (url) URL.revokeObjectURL(url);
    };
  }, [excalidrawTheme, isEmpty, scene]);

  useEffect(() => {
    if (!dialogOpen || Excalidraw) return;
    let mounted = true;
    void import('@excalidraw/excalidraw').then((mod) => {
      if (mounted) setExcalidraw(() => (mod as { Excalidraw: ExcalidrawComponent }).Excalidraw);
    });
    return () => {
      mounted = false;
    };
  }, [Excalidraw, dialogOpen]);

  const openEditor = useCallback(() => {
    const nextScene = sceneRef.current;
    draftSceneRef.current = nextScene;
    setEditorInitialScene({
      ...nextScene,
      appState: {
        ...nextScene.appState,
        theme: excalidrawTheme,
        viewBackgroundColor: 'transparent',
      },
    });
    setDialogOpen(true);
  }, [excalidrawTheme]);

  const saveDraft = useCallback(async () => {
    updateAttributes({ title, scene: normalizeExcalidrawScene(draftSceneRef.current) });
    if (!immediateSave) {
      setDialogOpen(false);
      return;
    }
    setSavingDraft(true);
    try {
      await new Promise((resolve) => {
        window.setTimeout(resolve, 0);
      });
      await immediateSave(editor.getJSON());
      setDialogOpen(false);
    } finally {
      setSavingDraft(false);
    }
  }, [editor, immediateSave, title, updateAttributes]);

  const cancelDraft = useCallback(() => {
    if (savingDraft) return;
    setDialogOpen(false);
  }, [savingDraft]);

  const downloadPng = useCallback(async () => {
    const file = await excalidrawPngFile(sceneRef.current, `${title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'excalidraw'}.png`, excalidrawTheme);
    const url = URL.createObjectURL(file);
    const link = document.createElement('a');
    link.href = url;
    link.download = file.name;
    link.click();
    URL.revokeObjectURL(url);
  }, [excalidrawTheme, title]);

  return (
    <NodeViewWrapper>
      <div className="not-prose my-6 rounded-md border border-border bg-background shadow-sm" contentEditable={false}>
        <div className="flex items-center justify-between gap-2 border-b border-border bg-muted/30 px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <SourceCodeIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate text-sm font-medium text-foreground">{title}</span>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {!isEmpty && (
              <QuickTooltip label="Download PNG">
                <button
                  type="button"
                  className="flex h-7 w-7 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-foreground"
                  onClick={(event) => {
                    event.stopPropagation();
                    void downloadPng();
                  }}
                >
                  <Download04Icon className="h-3.5 w-3.5" />
                </button>
              </QuickTooltip>
            )}
            {editable && (
              <>
                <QuickTooltip label="Edit drawing">
                  <button
                    type="button"
                    className="flex h-7 w-7 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-foreground"
                    onClick={(event) => {
                      event.stopPropagation();
                      openEditor();
                    }}
                  >
                    <PencilEdit01Icon className="h-3.5 w-3.5" />
                  </button>
                </QuickTooltip>
                <QuickTooltip label="Delete">
                  <button
                    type="button"
                    className="flex h-7 w-7 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-destructive"
                    onClick={(event) => {
                      event.stopPropagation();
                      deleteNode();
                    }}
                  >
                    <Delete01Icon className="h-3.5 w-3.5" />
                  </button>
                </QuickTooltip>
              </>
            )}
          </div>
        </div>
        <button
          type="button"
          className="block w-full bg-[linear-gradient(90deg,color-mix(in_oklch,var(--muted)_70%,transparent)_1px,transparent_1px),linear-gradient(color-mix(in_oklch,var(--muted)_70%,transparent)_1px,transparent_1px)] bg-[size:24px_24px] p-4 text-left"
          onClick={editable ? openEditor : undefined}
        >
          <div className="flex min-h-64 items-center justify-center overflow-hidden rounded border border-border/60 bg-background">
            {previewUrl ? (
              <img src={previewUrl} alt={title} className="max-h-[28rem] w-full object-contain" draggable={false} />
            ) : (
              <div className="px-6 py-10 text-center text-sm text-muted-foreground">
                {previewError || 'Open the drawing editor to create a diagram.'}
              </div>
            )}
          </div>
        </button>
      </div>

      <Dialog open={dialogOpen} onOpenChange={(open) => {
        if (savingDraft) return;
        setDialogOpen(open);
      }}>
        <DialogContent
          className="flex h-[calc(100vh-4rem)] w-[calc(100vw-4rem)] max-w-[1400px] flex-col gap-0 p-0 sm:max-w-[1400px]"
          onOpenAutoFocus={(event) => event.preventDefault()}
        >
          <DialogHeader className="border-b border-border px-4 py-3">
            <DialogTitle className="text-sm">{title}</DialogTitle>
          </DialogHeader>
          <div className="min-h-0 w-full flex-1 overflow-hidden">
            {Excalidraw ? (
              <Excalidraw
                key={dialogOpen ? 'open' : 'closed'}
                initialData={editorInitialScene}
                theme={excalidrawTheme}
                onChange={(elements: unknown[], appState: Record<string, unknown>, files: Record<string, unknown>) => {
                  draftSceneRef.current = normalizeExcalidrawScene({ elements, appState, files });
                }}
                UIOptions={{
                  canvasActions: {
                    loadScene: false,
                    saveToActiveFile: false,
                    export: false,
                    saveAsImage: false,
                  },
                  tools: { image: false },
                }}
              />
            ) : (
              <div className="flex h-full items-center justify-center text-sm text-muted-foreground">Loading drawing editor...</div>
            )}
          </div>
          <DialogFooter className="border-t border-border px-4 py-3">
            <Button type="button" variant="ghost" disabled={savingDraft} onClick={cancelDraft}>Cancel</Button>
            <Button type="button" disabled={savingDraft} onClick={() => { void saveDraft(); }}>
              {savingDraft ? 'Saving...' : 'Save drawing'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </NodeViewWrapper>
  );
}
