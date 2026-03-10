import { ICON_MAP } from '@/lib/icons'
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
  const Icon = ICON_MAP[name]
  if (Icon) {
    return <Icon size={size} weight={weight} className={className} />
  }
  // Emoji / text fallback for icons not in the curated map
  if (name) {
    return (
      <span className={className} style={{ fontSize: size * 0.85, lineHeight: 1 }}>
        {name}
      </span>
    )
  }
  return null
}
