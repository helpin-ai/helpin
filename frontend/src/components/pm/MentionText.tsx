import { useMemo, useState } from 'react'
import type { AssignableMember } from '@/lib/types'
import { UserAvatar } from '@/components/pm/UserAvatar'

interface MentionTextProps {
  text: string
  members?: AssignableMember[]
  className?: string
}

interface MentionMatch {
  handle: string
  member?: AssignableMember
}

function resolveMention(
  handle: string,
  membersByHandle: Map<string, AssignableMember>,
): MentionMatch {
  const member = membersByHandle.get(handle.toLowerCase())
  return { handle, member }
}

function MentionChip({ mention }: { mention: MentionMatch }) {
  const [showProfile, setShowProfile] = useState(false)

  return (
    <span
      className="relative inline"
      onMouseEnter={() => setShowProfile(true)}
      onMouseLeave={() => setShowProfile(false)}
    >
      <span className="cursor-pointer font-medium text-blue-600 dark:text-blue-400">
        @{mention.handle}
      </span>
      {showProfile && mention.member && (
        <span className="absolute bottom-full left-0 z-50 mb-1.5 flex items-center gap-2.5 whitespace-nowrap rounded-lg border border-border/60 bg-popover px-3 py-2 shadow-md">
          <UserAvatar
            name={mention.member.display_name}
            avatarUrl={mention.member.avatar_url}
            className="h-8 w-8 text-xs"
          />
          <span className="flex flex-col">
            <span className="text-sm font-semibold text-foreground">
              {mention.member.display_name}
            </span>
            <span className="text-xs text-muted-foreground">
              {mention.member.email}
            </span>
          </span>
        </span>
      )}
    </span>
  )
}

/**
 * Renders plain text with @mentions highlighted in blue.
 * Hovering a mention shows the user's profile card.
 */
export function MentionText({ text, members = [], className }: MentionTextProps) {
  const membersByHandle = useMemo(() => {
    const map = new Map<string, AssignableMember>()
    for (const m of members) {
      const handle = m.display_name.toLowerCase().replace(/\s+/g, '.')
      map.set(handle, m)
    }
    return map
  }, [members])

  const parts = useMemo(() => {
    const result: (string | MentionMatch)[] = []
    const regex = /@([a-z0-9._-]+)/gi
    let lastIndex = 0
    let match: RegExpExecArray | null

    while ((match = regex.exec(text)) !== null) {
      if (match.index > lastIndex) {
        result.push(text.slice(lastIndex, match.index))
      }
      result.push(resolveMention(match[1], membersByHandle))
      lastIndex = regex.lastIndex
    }

    if (lastIndex < text.length) {
      result.push(text.slice(lastIndex))
    }

    return result
  }, [text, membersByHandle])

  return (
    <span className={className}>
      {parts.map((part, i) =>
        typeof part === 'string' ? (
          part
        ) : (
          <MentionChip key={i} mention={part} />
        ),
      )}
    </span>
  )
}
