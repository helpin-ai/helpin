import { useEffect, useMemo, useRef, useState } from 'react';
import { ZoomIn, ZoomOut } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import {
  AVATAR_EDITOR_VIEWPORT_SIZE,
  clampAvatarTransform,
  canvasToFile,
  drawAvatarCropToCanvas,
  getAvatarBaseScale,
  getAvatarCropGeometry,
} from '@/lib/avatarCrop';
import type { AvatarTransform } from '@/lib/avatarCrop';

type AvatarCropDialogProps = {
  open: boolean;
  imageUrl: string | null;
  fileName: string;
  onOpenChange: (open: boolean) => void;
  onSave: (file: File) => Promise<void>;
  saving?: boolean;
};

type LoadedImage = {
  element: HTMLImageElement;
  width: number;
  height: number;
};

const PREVIEW_SIZE = 96;
const MIN_ZOOM = 1;
const MAX_ZOOM = 3;
const ZOOM_STEP = 0.01;

export function AvatarCropDialog({
  open,
  imageUrl,
  fileName,
  onOpenChange,
  onSave,
  saving = false,
}: AvatarCropDialogProps) {
  const [loadedImage, setLoadedImage] = useState<LoadedImage | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const [transform, setTransform] = useState<AvatarTransform>({ zoom: 1, offsetX: 0, offsetY: 0 });
  const dragOriginRef = useRef<{ pointerX: number; pointerY: number; offsetX: number; offsetY: number } | null>(null);
  const previewCanvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    if (!open || !imageUrl) {
      setLoadedImage(null);
      setLoadError(null);
      setTransform({ zoom: 1, offsetX: 0, offsetY: 0 });
      return;
    }

    let cancelled = false;
    const image = new Image();
    image.onload = () => {
      if (cancelled) return;
      setLoadedImage({ element: image, width: image.naturalWidth, height: image.naturalHeight });
      setLoadError(null);
      setTransform({ zoom: 1, offsetX: 0, offsetY: 0 });
    };
    image.onerror = () => {
      if (cancelled) return;
      setLoadedImage(null);
      setLoadError('Unable to load this image for cropping.');
    };
    image.src = imageUrl;

    return () => {
      cancelled = true;
    };
  }, [open, imageUrl]);

  const geometry = useMemo(() => {
    if (!loadedImage) {
      return null;
    }
    return getAvatarCropGeometry(loadedImage.width, loadedImage.height, transform, AVATAR_EDITOR_VIEWPORT_SIZE);
  }, [loadedImage, transform]);

  useEffect(() => {
    if (!loadedImage || !previewCanvasRef.current) {
      return;
    }
    drawAvatarCropToCanvas(
      loadedImage.element,
      loadedImage.width,
      loadedImage.height,
      transform,
      previewCanvasRef.current,
      AVATAR_EDITOR_VIEWPORT_SIZE,
      PREVIEW_SIZE,
    );
  }, [loadedImage, transform]);

  const setZoom = (nextZoom: number) => {
    if (!loadedImage) return;
    const normalizedZoom = Math.min(Math.max(nextZoom, MIN_ZOOM), MAX_ZOOM);
    setTransform((current) => clampAvatarTransform(
      loadedImage.width,
      loadedImage.height,
      { ...current, zoom: normalizedZoom },
      AVATAR_EDITOR_VIEWPORT_SIZE,
    ));
  };

  const handlePointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (!loadedImage) return;
    event.preventDefault();
    dragOriginRef.current = {
      pointerX: event.clientX,
      pointerY: event.clientY,
      offsetX: transform.offsetX,
      offsetY: transform.offsetY,
    };
    setDragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handlePointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    if (!loadedImage || !dragOriginRef.current) {
      return;
    }
    const deltaX = event.clientX - dragOriginRef.current.pointerX;
    const deltaY = event.clientY - dragOriginRef.current.pointerY;
    setTransform(clampAvatarTransform(
      loadedImage.width,
      loadedImage.height,
      {
        zoom: transform.zoom,
        offsetX: dragOriginRef.current.offsetX + deltaX,
        offsetY: dragOriginRef.current.offsetY + deltaY,
      },
      AVATAR_EDITOR_VIEWPORT_SIZE,
    ));
  };

  const handlePointerUp = (event: React.PointerEvent<HTMLDivElement>) => {
    dragOriginRef.current = null;
    setDragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  const handleReset = () => {
    setTransform({ zoom: 1, offsetX: 0, offsetY: 0 });
  };

  const handleSave = async () => {
    if (!loadedImage) {
      return;
    }

    const canvas = document.createElement('canvas');
    drawAvatarCropToCanvas(
      loadedImage.element,
      loadedImage.width,
      loadedImage.height,
      transform,
      canvas,
      AVATAR_EDITOR_VIEWPORT_SIZE,
    );
    const croppedFile = await canvasToFile(canvas, normalizeAvatarFileName(fileName));
    await onSave(croppedFile);
  };

  const zoomPercent = Math.round(transform.zoom * 100);
  const minRenderedScale = loadedImage ? Math.round(getAvatarBaseScale(loadedImage.width, loadedImage.height) * 100) : 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl gap-0 overflow-hidden p-0">
        <div className="grid min-h-[32rem] grid-cols-1 lg:grid-cols-[minmax(0,1fr)_19rem]">
          <div className="border-b border-border/60 bg-[#0b1020] p-6 text-white lg:border-r lg:border-b-0">
            <DialogHeader className="mb-5 text-left">
              <DialogTitle className="text-white">Adjust profile photo</DialogTitle>
              <DialogDescription className="text-white/70">
                Drag to reposition and zoom until the avatar feels balanced.
              </DialogDescription>
            </DialogHeader>

            <div className="flex flex-col items-center gap-5">
              <div
                className="relative h-[320px] w-[320px] overflow-hidden rounded-[28px] bg-black/60 touch-none"
                onPointerDown={handlePointerDown}
                onPointerMove={handlePointerMove}
                onPointerUp={handlePointerUp}
                onPointerCancel={handlePointerUp}
                style={{ cursor: dragging ? 'grabbing' : 'grab' }}
              >
                {loadedImage && geometry ? (
                  <>
                    <img
                      src={imageUrl ?? undefined}
                      alt="Avatar crop source"
                      draggable={false}
                      className="pointer-events-none absolute max-w-none select-none"
                      style={{
                        width: `${geometry.displayWidth}px`,
                        height: `${geometry.displayHeight}px`,
                        left: `${geometry.imageLeft}px`,
                        top: `${geometry.imageTop}px`,
                      }}
                    />
                    <div
                      className="pointer-events-none absolute inset-0 rounded-full border border-white/90 shadow-[0_0_0_9999px_rgba(3,7,18,0.56)]"
                      style={{ inset: '24px' }}
                    />
                    <div className="pointer-events-none absolute inset-0 rounded-[28px] ring-1 ring-white/10" />
                  </>
                ) : (
                  <div className="flex h-full items-center justify-center text-sm text-white/70">
                    {loadError ?? 'Loading image…'}
                  </div>
                )}
              </div>

              <div className="w-full max-w-md space-y-3">
                <div className="flex items-center justify-between text-xs text-white/70">
                  <span>Zoom</span>
                  <span>{zoomPercent}%</span>
                </div>
                <div className="flex items-center gap-3">
                  <ZoomOut className="h-4 w-4 shrink-0 text-white/60" />
                  <input
                    type="range"
                    min={MIN_ZOOM}
                    max={MAX_ZOOM}
                    step={ZOOM_STEP}
                    value={transform.zoom}
                    onChange={(event) => setZoom(Number(event.target.value))}
                    disabled={!loadedImage || saving}
                    className="h-1.5 w-full cursor-pointer appearance-none rounded-full bg-white/15 accent-white"
                  />
                  <ZoomIn className="h-4 w-4 shrink-0 text-white/60" />
                </div>
                <div className="flex items-center justify-between text-[11px] text-white/45">
                  <span>Fit: {minRenderedScale}%</span>
                  <button
                    type="button"
                    className="font-medium text-white/80 transition-colors hover:text-white disabled:cursor-not-allowed disabled:text-white/35"
                    onClick={handleReset}
                    disabled={!loadedImage || saving}
                  >
                    Reset
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div className="flex flex-col bg-background">
            <div className="flex-1 space-y-6 p-6">
              <div className="space-y-2">
                <p className="text-sm font-semibold text-foreground">Preview</p>
                <p className="text-xs text-muted-foreground">
                  This is how your avatar will appear around Helpin.
                </p>
              </div>

              <div className="flex items-center justify-center rounded-2xl border border-border/70 bg-muted/30 p-8">
                <canvas
                  ref={previewCanvasRef}
                  className="h-24 w-24 rounded-full border border-border/80 bg-muted object-cover shadow-sm"
                />
              </div>

              <div className="rounded-xl border border-border/70 bg-muted/25 p-4 text-sm">
                <p className="font-medium text-foreground">Tips</p>
                <ul className="mt-2 space-y-1 text-xs text-muted-foreground">
                  <li>Drag the image to center your face.</li>
                  <li>Use zoom to tighten the crop without losing quality.</li>
                  <li>The uploaded avatar is exported as a square high-res image.</li>
                </ul>
              </div>
            </div>

            <DialogFooter className="border-t border-border/60 px-6 py-4 sm:justify-between">
              <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={saving}>
                Cancel
              </Button>
              <Button onClick={() => void handleSave()} disabled={!loadedImage || !!loadError || saving}>
                {saving ? 'Saving…' : 'Save photo'}
              </Button>
            </DialogFooter>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function normalizeAvatarFileName(fileName: string) {
  const baseName = fileName.replace(/\.[^.]+$/, '').trim() || 'avatar';
  return `${baseName}.png`;
}
