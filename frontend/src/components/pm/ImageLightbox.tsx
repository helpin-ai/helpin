import { Dialog as DialogPrimitive } from 'radix-ui'
import { Cancel01Icon } from '@/lib/icons'

interface ImageLightboxProps {
  src: string
  alt?: string
  kind?: 'image' | 'video'
  onClose: () => void
}

export function ImageLightbox({ src, alt, kind = 'image', onClose }: ImageLightboxProps) {
  return (
    <DialogPrimitive.Root open onOpenChange={(next: boolean) => { if (!next) onClose() }}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-[9999] bg-black/70 backdrop-blur-sm data-[state=open]:animate-in data-[state=open]:fade-in-0" />
        <DialogPrimitive.Content
          aria-describedby={undefined}
          className="fixed inset-0 z-[9999] flex items-center justify-center outline-none"
          onOpenAutoFocus={(e: Event) => e.preventDefault()}
        >
          <DialogPrimitive.Title className="sr-only">{alt || (kind === 'video' ? 'Video preview' : 'Image preview')}</DialogPrimitive.Title>
          <DialogPrimitive.Close
            className="absolute top-4 right-4 inline-flex h-9 w-9 items-center justify-center rounded-full bg-black/40 text-white hover:bg-black/60 cursor-pointer"
            aria-label="Close preview"
          >
            <Cancel01Icon className="h-5 w-5" />
          </DialogPrimitive.Close>
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
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}
