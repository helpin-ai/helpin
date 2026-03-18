import { forwardRef, useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'

import { cn } from '@/lib/utils'

interface LoadingImageProps extends React.ImgHTMLAttributes<HTMLImageElement> {
  containerClassName?: string
  spinnerClassName?: string
}

export const LoadingImage = forwardRef<HTMLImageElement, LoadingImageProps>(function LoadingImage(
  { containerClassName, spinnerClassName, className, src, onLoad, onError, ...props },
  ref,
) {
  const [isLoading, setIsLoading] = useState(Boolean(src))

  useEffect(() => {
    setIsLoading(Boolean(src))
  }, [src])

  return (
    <span className={cn('relative inline-flex max-w-full align-top', containerClassName)}>
      {isLoading && (
        <span
          data-loading-image-spinner="true"
          className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-[inherit] bg-muted/60 backdrop-blur-[1px]"
        >
          <Loader2 className={cn('h-4 w-4 animate-spin text-muted-foreground', spinnerClassName)} />
        </span>
      )}

      <img
        {...props}
        ref={ref}
        src={src}
        className={className}
        onLoad={(event) => {
          setIsLoading(false)
          onLoad?.(event)
        }}
        onError={(event) => {
          setIsLoading(false)
          onError?.(event)
        }}
      />
    </span>
  )
})
