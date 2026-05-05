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
  const singleName = names[0]

  if (ids.length === 0) {
    return null
  }

  if (ids.length === 1) {
    return (
      <div
        data-owner-avatar-stack
        title={singleName}
        className={cn('inline-flex min-w-0 items-center gap-1.5', className)}
        aria-label={singleName}
      >
        <span data-owner-avatar className="inline-flex shrink-0">
          <UserAvatar name={singleName} className={avatarSize} />
        </span>
        <span data-owner-single-name className="min-w-0 truncate">
          {singleName}
        </span>
      </div>
    )
  }

  return (
    <div
      data-owner-avatar-stack
      title={names.join(', ')}
      className={cn('group/owners relative inline-flex items-center pl-2', className)}
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
      <div
        data-owner-hover-list
        className="pointer-events-none absolute right-0 top-full z-50 mt-1 hidden min-w-44 rounded-md border border-border bg-popover p-1.5 text-popover-foreground shadow-md group-hover/owners:block group-focus-within/owners:block"
      >
        <div className="flex flex-col gap-1">
          {ids.map((id) => {
            const name = nameMap?.get(id) ?? 'Unknown owner'
            return (
              <div key={id} className="flex min-w-0 items-center gap-2">
                <UserAvatar name={name} className="h-5 w-5" />
                <span className="truncate text-xs">{name}</span>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
