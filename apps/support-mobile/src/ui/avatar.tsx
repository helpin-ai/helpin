import { useState } from 'react'
import { cn } from '@mobile/lib/cn'

export interface AvatarProps {
  name: string
  src?: string
  size?: number
  className?: string
  /** Number of name initials shown by the fallback. Support lists use one to match web. */
  initialCount?: 1 | 2
}

/** First letters of the first two words of `name`, uppercased (e.g. "Ada Lovelace" -> "AL"). */
export function getInitials(name: string, count: 1 | 2 = 2): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  return words
    .slice(0, count)
    .map((word) => word[0])
    .join('')
    .toUpperCase()
}

export function Avatar({ name, src, size = 40, className, initialCount = 2 }: AvatarProps) {
  const [imageFailed, setImageFailed] = useState(false)
  const showImage = Boolean(src) && !imageFailed

  return (
    <span
      className={cn(
        'relative inline-flex shrink-0 select-none items-center justify-center overflow-hidden rounded-full bg-muted text-muted-foreground',
        className,
      )}
      style={{ width: size, height: size }}
    >
      {showImage ? (
        <img
          src={src}
          alt={name}
          className="h-full w-full object-cover"
          onError={() => setImageFailed(true)}
        />
      ) : (
        <span
          className="font-medium"
          style={{ fontSize: initialCount === 1 ? Math.max(9, size * 0.34) : Math.max(10, size * 0.4) }}
        >
          {getInitials(name, initialCount)}
        </span>
      )}
    </span>
  )
}
