import { useEffect, useState } from 'react'
import { getAvatarColor } from '@/components/support/helpers'
import { cn } from '@mobile/lib/cn'
import { Avatar, type AvatarProps } from './avatar'

export interface TeamMemberAvatarIdentity {
  id?: string | null
  user_id?: string | null
  avatar_url?: string | null
  avatar_style?: string | null
  avatar_seed?: string | null
  avatar_background_mode?: string | null
  avatar_background_color?: string | null
}

export interface TeamMemberAvatarProps extends Omit<AvatarProps, 'src'> {
  member?: TeamMemberAvatarIdentity | null
  /** A message/presence-provided image takes precedence over the member profile. */
  src?: string | null
  fallbackSeed?: string | null
}

/** Uploaded image, generated profile avatar, then deterministic initials. */
export function TeamMemberAvatar({
  member,
  name,
  src,
  fallbackSeed,
  className,
  ...avatarProps
}: TeamMemberAvatarProps) {
  const colorSeed = fallbackSeed || member?.user_id || member?.id || name
  const uploadedSrc = src?.trim() || member?.avatar_url?.trim() || undefined
  const [resolvedSrc, setResolvedSrc] = useState<string | undefined>(uploadedSrc)

  useEffect(() => {
    let active = true
    if (uploadedSrc) {
      setResolvedSrc(uploadedSrc)
      return () => { active = false }
    }
    if (!member?.avatar_style?.trim() || !member.avatar_seed?.trim()) {
      setResolvedSrc(undefined)
      return () => { active = false }
    }

    // DiceBear's style collection is intentionally deferred: most teammates
    // use an uploaded photo or initials, so it should not tax mobile startup.
    void import('@/lib/teamMemberAvatar').then(({ resolveTeamMemberAvatarSrc }) => {
      if (!active) return
      setResolvedSrc(resolveTeamMemberAvatarSrc({
        avatarStyle: member.avatar_style,
        avatarSeed: member.avatar_seed,
        avatarBackgroundMode: member.avatar_background_mode,
        avatarBackgroundColor: member.avatar_background_color,
        fallbackSeed: colorSeed,
      }))
    })

    return () => { active = false }
  }, [
    colorSeed,
    member?.avatar_background_color,
    member?.avatar_background_mode,
    member?.avatar_seed,
    member?.avatar_style,
    uploadedSrc,
  ])

  return (
    <Avatar
      {...avatarProps}
      name={name}
      src={resolvedSrc}
      className={cn(getAvatarColor(colorSeed), className)}
    />
  )
}
