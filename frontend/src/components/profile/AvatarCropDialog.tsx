import { useEffect, useMemo, useRef, useState } from 'react';
import { ZoomIn, ZoomOut } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import {
  clampAvatarTransform,
  canvasToFile,
  drawAvatarCropToCanvas,
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

const CROP_VIEWPORT = 280;
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
    return getAvatarCropGeometry(loadedImage.width, loadedImage.height, transform, CROP_VIEWPORT);
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
      CROP_VIEWPORT,
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
      CROP_VIEWPORT,
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
      CROP_VIEWPORT,
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
      CROP_VIEWPORT,
    );
    const croppedFile = await canvasToFile(canvas, normalizeAvatarFileName(fileName));
    await onSave(croppedFile);
  };

  const zoomPercent = Math.round(transform.zoom * 100);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl gap-0 overflow-hidden p-0">
        <div className="p-6 pb-0">
          <DialogHeader className="text-left">
            <DialogTitle>Adjust profile photo</DialogTitle>
            <DialogDescription>
              Drag to reposition and zoom until the avatar feels balanced.
            </DialogDescription>
          </DialogHeader>
        </div>

        <div className="flex flex-col items-center gap-6 p-6">
          <div className="flex w-full items-start justify-center gap-8">
            {/* Crop viewport */}
            <div className="flex flex-col items-center gap-4">
              <div
                className="relative h-[280px] w-[280px] overflow-hidden rounded-full bg-muted touch-none ring-1 ring-border"
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
                  </>
                ) : (
                  <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                    {loadError ?? 'Loading image…'}
                  </div>
                )}
              </div>
            </div>

            {/* Preview */}
            <div className="flex flex-col items-center gap-4 pt-4">
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Preview</p>
              <canvas
                ref={previewCanvasRef}
                className="h-20 w-20 rounded-full border border-border bg-muted object-cover shadow-sm"
              />
              <p className="text-[11px] text-muted-foreground/70">How others see you</p>
            </div>
          </div>

          {/* Zoom controls */}
          <div className="w-full max-w-sm space-y-2">
            <div className="flex items-center gap-3">
              <ZoomOut className="h-4 w-4 shrink-0 text-muted-foreground" />
              <input
                type="range"
                min={MIN_ZOOM}
                max={MAX_ZOOM}
                step={ZOOM_STEP}
                value={transform.zoom}
                onChange={(event) => setZoom(Number(event.target.value))}
                disabled={!loadedImage || saving}
                className="h-1.5 w-full cursor-pointer appearance-none rounded-full bg-muted accent-primary"
              />
              <ZoomIn className="h-4 w-4 shrink-0 text-muted-foreground" />
            </div>
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>{zoomPercent}%</span>
              <button
                type="button"
                className="font-medium text-foreground/70 transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:text-muted-foreground/50"
                onClick={handleReset}
                disabled={!loadedImage || saving}
              >
                Reset
              </button>
            </div>
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
      </DialogContent>
    </Dialog>
  );
}

function normalizeAvatarFileName(fileName: string) {
  const baseName = fileName.replace(/\.[^.]+$/, '').trim() || 'avatar';
  return `${baseName}.png`;
}
