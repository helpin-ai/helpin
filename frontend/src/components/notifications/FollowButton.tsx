import { NotificationOff02Icon, Notification02Icon } from '@/lib/icons'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useIsFollowing, useFollowEntity, useUnfollowEntity } from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

interface FollowButtonProps {
  entityType: string
  entityId: string
  size?: 'sm' | 'icon'
}

export function FollowButton({ entityType, entityId, size = 'icon' }: FollowButtonProps) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id || ''

  const { data } = useIsFollowing(wsId, entityType, entityId)
  const follow = useFollowEntity(wsId)
  const unfollow = useUnfollowEntity(wsId)

  const isFollowing = data?.following ?? false

  const handleToggle = () => {
    if (isFollowing) {
      unfollow.mutate({ entityType, entityId })
    } else {
      follow.mutate({ entityType, entityId })
    }
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size={size}
          className={size === 'icon' ? 'h-7 w-7' : 'h-7 gap-1.5 text-xs'}
          onClick={handleToggle}
          disabled={follow.isPending || unfollow.isPending}
        >
          {isFollowing ? (
            <>
              <Notification02Icon className="h-3.5 w-3.5 text-primary" />
              {size === 'sm' && <span>Following</span>}
            </>
          ) : (
            <>
              <NotificationOff02Icon className="h-3.5 w-3.5 text-muted-foreground" />
              {size === 'sm' && <span>Follow</span>}
            </>
          )}
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        {isFollowing ? 'Unfollow (stop notifications)' : 'Follow (get notifications)'}
      </TooltipContent>
    </Tooltip>
  )
}
