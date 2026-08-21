import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type WheelEvent } from 'react'
import { ChevronLeft, ChevronRight, Download, ExternalLink, Share2, X } from 'lucide-react'
import { openUrl } from '@tauri-apps/plugin-opener'
import type { SupportAttachmentPayload } from '@helpin-ai/support-core'
import { toast } from 'sonner'
import { isTauri } from '@mobile/lib/host'
import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'

interface ImageViewerProps {
  images: SupportAttachmentPayload[]
  initialIndex: number
  open: boolean
  onOpenChange: (open: boolean) => void
}

interface Point {
  x: number
  y: number
}

interface ViewTransform {
  scale: number
  x: number
  y: number
}

type Gesture =
  | { kind: 'single'; start: Point; transform: ViewTransform }
  | { kind: 'pinch'; distance: number; transform: ViewTransform }

const MIN_SCALE = 1
const MAX_SCALE = 4
const DOUBLE_TAP_SCALE = 2.5
const SWIPE_THRESHOLD_PX = 56

const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value))
const distance = (a: Point, b: Point) => Math.hypot(a.x - b.x, a.y - b.y)

async function openExternally(url: string): Promise<void> {
  if (isTauri()) {
    await openUrl(url)
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

export function ImageViewer({ images, initialIndex, open, onOpenChange }: ImageViewerProps) {
  const [activeIndex, setActiveIndex] = useState(initialIndex)
  const [transform, setTransform] = useState<ViewTransform>({ scale: 1, x: 0, y: 0 })
  const [swipeX, setSwipeX] = useState(0)
  const [interacting, setInteracting] = useState(false)
  const pointersRef = useRef(new Map<number, Point>())
  const gestureRef = useRef<Gesture | null>(null)
  const lastTapAtRef = useRef(0)

  const image = images[activeIndex]

  const resetTransform = () => {
    setTransform({ scale: 1, x: 0, y: 0 })
    setSwipeX(0)
  }

  useEffect(() => {
    if (!open) return
    setActiveIndex(clamp(initialIndex, 0, Math.max(0, images.length - 1)))
    resetTransform()
  }, [images.length, initialIndex, open])

  useEffect(() => {
    resetTransform()
  }, [activeIndex])

  if (!image) return null

  const showImage = (nextIndex: number) => {
    setActiveIndex(clamp(nextIndex, 0, images.length - 1))
  }

  const toggleZoom = () => {
    setTransform((current) =>
      current.scale > MIN_SCALE
        ? { scale: MIN_SCALE, x: 0, y: 0 }
        : { scale: DOUBLE_TAP_SCALE, x: 0, y: 0 },
    )
  }

  const beginPinch = () => {
    const points = [...pointersRef.current.values()]
    if (points.length < 2) return
    gestureRef.current = {
      kind: 'pinch',
      distance: Math.max(1, distance(points[0], points[1])),
      transform,
    }
  }

  const handlePointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.stopPropagation()
    event.currentTarget.setPointerCapture(event.pointerId)
    pointersRef.current.set(event.pointerId, { x: event.clientX, y: event.clientY })
    setInteracting(true)
    if (pointersRef.current.size === 1) {
      gestureRef.current = {
        kind: 'single',
        start: { x: event.clientX, y: event.clientY },
        transform,
      }
    } else if (pointersRef.current.size === 2) {
      beginPinch()
    }
  }

  const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.stopPropagation()
    if (!pointersRef.current.has(event.pointerId)) return
    pointersRef.current.set(event.pointerId, { x: event.clientX, y: event.clientY })

    if (pointersRef.current.size >= 2) {
      if (gestureRef.current?.kind !== 'pinch') beginPinch()
      const gesture = gestureRef.current
      const points = [...pointersRef.current.values()]
      if (gesture?.kind !== 'pinch' || points.length < 2) return
      const nextScale = clamp(
        gesture.transform.scale * (distance(points[0], points[1]) / gesture.distance),
        MIN_SCALE,
        MAX_SCALE,
      )
      setTransform({
        scale: nextScale,
        x: nextScale === MIN_SCALE ? 0 : gesture.transform.x,
        y: nextScale === MIN_SCALE ? 0 : gesture.transform.y,
      })
      setSwipeX(0)
      return
    }

    const gesture = gestureRef.current
    if (gesture?.kind !== 'single') return
    const dx = event.clientX - gesture.start.x
    const dy = event.clientY - gesture.start.y
    if (gesture.transform.scale > MIN_SCALE) {
      setTransform({
        ...gesture.transform,
        x: gesture.transform.x + dx,
        y: gesture.transform.y + dy,
      })
    } else {
      setSwipeX(dx)
    }
  }

  const finishPointer = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.stopPropagation()
    const gesture = gestureRef.current
    const wasLastPointer = pointersRef.current.size === 1
    pointersRef.current.delete(event.pointerId)

    if (!wasLastPointer) {
      const remaining = [...pointersRef.current.values()][0]
      if (remaining) gestureRef.current = { kind: 'single', start: remaining, transform }
      return
    }

    if (gesture?.kind === 'single') {
      const dx = event.clientX - gesture.start.x
      const dy = event.clientY - gesture.start.y
      if (
        gesture.transform.scale === MIN_SCALE &&
        Math.abs(dx) >= SWIPE_THRESHOLD_PX &&
        Math.abs(dx) > Math.abs(dy)
      ) {
        if (dx < 0 && activeIndex < images.length - 1) showImage(activeIndex + 1)
        if (dx > 0 && activeIndex > 0) showImage(activeIndex - 1)
      } else if (Math.abs(dx) < 10 && Math.abs(dy) < 10) {
        const now = Date.now()
        if (now - lastTapAtRef.current < 300) {
          toggleZoom()
          lastTapAtRef.current = 0
        } else {
          lastTapAtRef.current = now
        }
      }
    }

    gestureRef.current = null
    setSwipeX(0)
    setInteracting(false)
  }

  const handleWheel = (event: WheelEvent<HTMLDivElement>) => {
    event.preventDefault()
    const nextScale = clamp(transform.scale - event.deltaY * 0.002, MIN_SCALE, MAX_SCALE)
    setTransform({
      scale: nextScale,
      x: nextScale === MIN_SCALE ? 0 : transform.x,
      y: nextScale === MIN_SCALE ? 0 : transform.y,
    })
  }

  const shareImage = async () => {
    try {
      if (typeof navigator.share === 'function') {
        await navigator.share({ title: image.file_name, url: image.url })
        return
      }
      await navigator.clipboard.writeText(image.url)
      toast.success('Image link copied')
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') return
      toast.error('Could not share this image')
    }
  }

  const downloadImage = async () => {
    try {
      const response = await fetch(image.url)
      if (!response.ok) throw new Error('Image download failed')
      const objectUrl = URL.createObjectURL(await response.blob())
      const link = document.createElement('a')
      link.href = objectUrl
      link.download = image.file_name || 'image'
      document.body.appendChild(link)
      link.click()
      link.remove()
      setTimeout(() => URL.revokeObjectURL(objectUrl), 0)
      toast.success('Image downloaded')
    } catch {
      toast.error('Could not download this image')
    }
  }

  const handleOpenExternal = async () => {
    try {
      await openExternally(image.url)
    } catch {
      toast.error('Could not open this image')
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      fullScreen
      title={image.file_name || 'Image preview'}
      className="bg-black text-white"
    >
      <header className="flex h-14 shrink-0 items-center gap-2 px-2">
        <Pressable
          aria-label="Close image viewer"
          haptic="selection"
          onPress={() => onOpenChange(false)}
          className="flex items-center justify-center rounded-full text-white active:bg-white/10"
        >
          <X className="h-6 w-6" />
        </Pressable>
        <div className="min-w-0 flex-1 text-center">
          <p className="truncate text-body font-medium">{image.file_name}</p>
          {images.length > 1 && (
            <p className="text-xs text-white/60">{activeIndex + 1} of {images.length}</p>
          )}
        </div>
        <span aria-hidden className="h-11 w-11" />
      </header>

      <div
        data-vaul-no-drag
        data-testid="image-viewer-stage"
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={finishPointer}
        onPointerCancel={finishPointer}
        onWheel={handleWheel}
        className="relative min-h-0 flex-1 touch-none select-none overflow-hidden"
      >
        <img
          key={image.id}
          src={image.url}
          alt={image.file_name}
          draggable={false}
          className="pointer-events-none h-full w-full object-contain will-change-transform"
          style={{
            transform: `translate3d(${transform.x + swipeX}px, ${transform.y}px, 0) scale(${transform.scale})`,
            transition: interacting ? 'none' : 'transform 180ms ease-out',
          }}
        />

        {images.length > 1 && activeIndex > 0 && transform.scale === MIN_SCALE && (
          <Pressable
            aria-label="Previous image"
            haptic="selection"
            onPress={() => showImage(activeIndex - 1)}
            className="absolute left-2 top-1/2 flex -translate-y-1/2 items-center justify-center rounded-full bg-black/45 text-white backdrop-blur-sm"
          >
            <ChevronLeft className="h-6 w-6" />
          </Pressable>
        )}
        {images.length > 1 && activeIndex < images.length - 1 && transform.scale === MIN_SCALE && (
          <Pressable
            aria-label="Next image"
            haptic="selection"
            onPress={() => showImage(activeIndex + 1)}
            className="absolute right-2 top-1/2 flex -translate-y-1/2 items-center justify-center rounded-full bg-black/45 text-white backdrop-blur-sm"
          >
            <ChevronRight className="h-6 w-6" />
          </Pressable>
        )}
      </div>

      <footer className="flex shrink-0 items-center justify-center gap-5 border-t border-white/10 px-4 py-2">
        <Pressable
          aria-label="Share image"
          haptic="selection"
          onPress={() => void shareImage()}
          className="flex flex-col items-center justify-center gap-0.5 rounded-xl px-3 text-xs text-white/85 active:bg-white/10"
        >
          <Share2 className="h-5 w-5" />
          <span>Share</span>
        </Pressable>
        <Pressable
          aria-label="Download image"
          haptic="selection"
          onPress={() => void downloadImage()}
          className="flex flex-col items-center justify-center gap-0.5 rounded-xl px-3 text-xs text-white/85 active:bg-white/10"
        >
          <Download className="h-5 w-5" />
          <span>Save</span>
        </Pressable>
        <Pressable
          aria-label="Open image externally"
          haptic="selection"
          onPress={() => void handleOpenExternal()}
          className="flex flex-col items-center justify-center gap-0.5 rounded-xl px-3 text-xs text-white/85 active:bg-white/10"
        >
          <ExternalLink className="h-5 w-5" />
          <span>Open</span>
        </Pressable>
      </footer>
    </Sheet>
  )
}
