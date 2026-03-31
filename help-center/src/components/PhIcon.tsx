import { Suspense } from 'react'
import { getIconComponent } from '@/lib/icons'
import type { IconWeight } from '@phosphor-icons/react'

interface PhIconProps {
  name: string
  size?: number
  weight?: IconWeight
  className?: string
}

export function PhIcon({
  name,
  size = 20,
  weight = 'regular',
  className,
}: PhIconProps) {
  const Icon = getIconComponent(name)

  if (Icon) {
    return (
      <Suspense
        fallback={
          <span
            className={className}
            style={{ display: 'inline-block', width: size, height: size }}
          />
        }
      >
        <Icon size={size} weight={weight} className={className} />
      </Suspense>
    )
  }

  // Emoji / text fallback for icons not in Phosphor
  if (name) {
    return (
      <span className={className} style={{ fontSize: size * 0.85, lineHeight: 1 }}>
        {name}
      </span>
    )
  }

  return null
}
