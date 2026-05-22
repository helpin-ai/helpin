import { Dialog as DialogPrimitive } from 'radix-ui'
import type { KeyboardEvent } from 'react'
import { ArrowLeft01Icon, ArrowRight01Icon, Cancel01Icon } from '@/lib/icons'

interface ImageLightboxProps {
  src: string
  alt?: string
  kind?: 'image' | 'video'
  onClose: () => void
  onPrevious?: () => void
  onNext?: () => void
  hasPrevious?: boolean
  hasNext?: boolean
  positionLabel?: string
}

export function ImageLightbox({
  src,
  alt,
  kind = 'image',
  onClose,
  onPrevious,
  onNext,
  hasPrevious = false,
  hasNext = false,
  positionLabel,
}: ImageLightboxProps) {
  return (
    <DialogPrimitive.Root open onOpenChange={(next: boolean) => { if (!next) onClose() }}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-[9999] bg-black/70 backdrop-blur-sm data-[state=open]:animate-in data-[state=open]:fade-in-0" />
        <DialogPrimitive.Content
          aria-describedby={undefined}
          className="fixed inset-0 z-[9999] flex items-center justify-center outline-none"
          onOpenAutoFocus={(e: Event) => e.preventDefault()}
          onKeyDown={(event: KeyboardEvent) => {
            if (event.key === 'ArrowLeft' && hasPrevious) {
              event.preventDefault()
              onPrevious?.()
            } else if (event.key === 'ArrowRight' && hasNext) {
              event.preventDefault()
              onNext?.()
            }
          }}
        >
          <DialogPrimitive.Title className="sr-only">{alt || (kind === 'video' ? 'Video preview' : 'Image preview')}</DialogPrimitive.Title>
          <DialogPrimitive.Close
            className="absolute top-4 right-4 inline-flex h-9 w-9 items-center justify-center rounded-full bg-black/40 text-white hover:bg-black/60 cursor-pointer"
            aria-label="Close preview"
          >
            <Cancel01Icon className="h-5 w-5" />
          </DialogPrimitive.Close>
          {hasPrevious && (
            <button
              type="button"
              className="absolute left-4 top-1/2 inline-flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-md bg-black/40 text-white hover:bg-black/60 cursor-pointer"
              aria-label="Previous attachment"
              onClick={(event) => {
                event.stopPropagation()
                onPrevious?.()
              }}
            >
              <ArrowLeft01Icon className="h-6 w-6" />
            </button>
          )}
          {hasNext && (
            <button
              type="button"
              className="absolute right-4 top-1/2 inline-flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-md bg-black/40 text-white hover:bg-black/60 cursor-pointer"
              aria-label="Next attachment"
              onClick={(event) => {
                event.stopPropagation()
                onNext?.()
              }}
            >
              <ArrowRight01Icon className="h-6 w-6" />
            </button>
          )}
          {kind === 'video' ? (
            <video
              src={src}
              controls
              autoPlay
              className="max-h-[90vh] max-w-[90vw] rounded-lg bg-black shadow-2xl"
            >
              <a href={src} target="_blank" rel="noopener noreferrer">{alt || 'Open video'}</a>
            </video>
          ) : (
            <img
              src={src}
              alt={alt ?? ''}
              className="max-h-[90vh] max-w-[90vw] rounded-lg object-contain shadow-2xl"
            />
          )}
          {positionLabel ? (
            <div className="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-md bg-black/40 px-2 py-1 text-xs text-white/75">
              {positionLabel}
            </div>
          ) : null}
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}
