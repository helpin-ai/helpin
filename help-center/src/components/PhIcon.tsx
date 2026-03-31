import { getIconComponent } from '@/lib/icons'

interface PhIconProps {
  name: string
  size?: number
  weight?: string
  className?: string
}

export function PhIcon({
  name,
  size = 20,
  className,
}: PhIconProps) {
  const Icon = getIconComponent(name)

  if (Icon) {
    return <Icon size={size} className={className} />
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
