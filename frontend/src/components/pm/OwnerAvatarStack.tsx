import { UserAvatar } from './UserAvatar'
import { cn } from '@/lib/utils'

interface OwnerAvatarStackProps {
  memberIds: string[]
  nameMap?: Map<string, string>
  size?: 'sm' | 'md'
  max?: number
  className?: string
}

export function OwnerAvatarStack({ memberIds, nameMap, size = 'sm', max = 3, className }: OwnerAvatarStackProps) {
  const ids = memberIds.filter(Boolean)
  const visibleIds = ids.slice(0, max)
  const overflow = Math.max(0, ids.length - visibleIds.length)
  const names = ids.map((id) => nameMap?.get(id) ?? 'Unknown owner')
  const avatarSize = size === 'md' ? 'h-7 w-7' : 'h-6 w-6'

  if (ids.length === 0) {
    return null
  }

  return (
    <div
      data-owner-avatar-stack
      title={names.join(', ')}
      className={cn('inline-flex items-center pl-2', className)}
      aria-label={names.join(', ')}
    >
      {visibleIds.map((id, index) => (
        <span key={id} data-owner-avatar className={cn('inline-flex', index > 0 && '-ml-2')}>
          <UserAvatar name={nameMap?.get(id) ?? 'Unknown owner'} className={avatarSize} />
        </span>
      ))}
      {overflow > 0 ? (
        <span
          className={cn(
            '-ml-2 inline-flex items-center justify-center rounded-full border border-background bg-muted text-[10px] font-medium text-muted-foreground',
            avatarSize,
          )}
        >
          +{overflow}
        </span>
      ) : null}
    </div>
  )
}
