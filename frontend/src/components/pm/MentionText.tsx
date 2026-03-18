import { useMemo, useState } from 'react'
import type { AssignableMember, WorkspaceTeam } from '@/lib/types'
import { UserAvatar } from '@/components/pm/UserAvatar'
import { getMemberMentionHandle } from '@/components/pm/mentionSuggestions'

interface MentionTextProps {
  text: string
  members?: AssignableMember[]
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[]
  className?: string
}

interface MentionMatch {
  handle: string
  type: 'person' | 'team' | 'unresolved'
  member?: AssignableMember
  team?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>
}

function resolveMention(
  handle: string,
  membersByHandle: Map<string, AssignableMember>,
  teamsByHandle: Map<string, Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>>,
): MentionMatch {
  const team = teamsByHandle.get(handle.toLowerCase())
  const member = membersByHandle.get(handle.toLowerCase())
  if (team) return { handle, type: 'team', team }
  if (member) return { handle, type: 'person', member }
  return { handle, type: 'unresolved' }
}

function MentionChip({ mention }: { mention: MentionMatch }) {
  const [showProfile, setShowProfile] = useState(false)
  const isTeam = mention.type === 'team'

  return (
    <span
      className="relative inline-flex"
      data-mention-type={mention.type}
      onMouseEnter={() => setShowProfile(true)}
      onMouseLeave={() => setShowProfile(false)}
    >
      <span
        className={`cursor-pointer font-medium ${
          isTeam
            ? 'rounded-md border border-emerald-500/20 bg-emerald-500/10 px-1.5 py-0.5 text-emerald-700 dark:text-emerald-300'
            : 'text-blue-600 dark:text-blue-400'
        }`}
      >
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
      {showProfile && mention.team && (
        <span className="absolute bottom-full left-0 z-50 mb-1.5 flex items-center gap-2.5 whitespace-nowrap rounded-lg border border-border/60 bg-popover px-3 py-2 shadow-md">
          <span className="flex h-8 w-8 items-center justify-center rounded-full bg-emerald-500/10 text-xs font-semibold text-emerald-700 dark:text-emerald-300">
            {mention.team.name.slice(0, 1).toUpperCase()}
          </span>
          <span className="flex flex-col">
            <span className="text-sm font-semibold text-foreground">
              {mention.team.name}
            </span>
            <span className="text-xs text-muted-foreground">
              @{mention.team.handle}
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
export function MentionText({ text, members = [], teams = [], className }: MentionTextProps) {
  const membersByHandle = useMemo(() => {
    const map = new Map<string, AssignableMember>()
    for (const m of members) {
      const handle = getMemberMentionHandle(m)
      map.set(handle, m)
    }
    return map
  }, [members])

  const teamsByHandle = useMemo(() => {
    const map = new Map<string, Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>>()
    for (const team of teams) {
      if (!team.handle) continue
      map.set(team.handle.toLowerCase(), team)
    }
    return map
  }, [teams])

  const parts = useMemo(() => {
    const result: (string | MentionMatch)[] = []
    const regex = /@([a-z0-9._-]+)/gi
    let lastIndex = 0
    let match: RegExpExecArray | null

    while ((match = regex.exec(text)) !== null) {
      if (match.index > lastIndex) {
        result.push(text.slice(lastIndex, match.index))
      }
      result.push(resolveMention(match[1], membersByHandle, teamsByHandle))
      lastIndex = regex.lastIndex
    }

    if (lastIndex < text.length) {
      result.push(text.slice(lastIndex))
    }

    return result
  }, [text, membersByHandle, teamsByHandle])

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
