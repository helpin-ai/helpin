import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { cn, getInitials } from '@/lib/utils';

interface UserAvatarProps {
  name?: string | null;
  className?: string;
  fallbackClassName?: string;
}

export function UserAvatar({
  name,
  className,
  fallbackClassName,
}: UserAvatarProps) {
  return (
    <Avatar className={cn('h-6 w-6 border border-border/80', className)}>
      <AvatarFallback className={cn('text-[9px] font-semibold bg-muted/60', fallbackClassName)}>
        {getInitials(name)}
      </AvatarFallback>
    </Avatar>
  );
}
