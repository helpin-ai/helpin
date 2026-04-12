import { useCallback, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
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
  const [visible, setVisible] = useState(false)
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null)
  const ref = useRef<HTMLSpanElement>(null)
  const hideTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isTeam = mention.type === 'team'

  const show = useCallback(() => {
    if (hideTimer.current) { clearTimeout(hideTimer.current); hideTimer.current = null }
    if (ref.current) {
      const rect = ref.current.getBoundingClientRect()
      setPos({ top: rect.top - 6, left: rect.left })
    }
    setVisible(true)
  }, [])

  const hide = useCallback(() => {
    hideTimer.current = setTimeout(() => setVisible(false), 120)
  }, [])

  const profileCard = visible && pos && (mention.member || mention.team) ? createPortal(
    <span
      className="fixed z-[9999] flex items-center gap-2.5 whitespace-nowrap rounded-lg border border-border/60 bg-popover px-3 py-2 shadow-md animate-in fade-in-0 zoom-in-95 duration-150"
      style={{ top: pos.top, left: pos.left, transform: 'translateY(-100%)' }}
      onMouseEnter={show}
      onMouseLeave={hide}
    >
      {mention.member && (
        <>
          <UserAvatar
            name={mention.member.display_name}
            avatarUrl={mention.member.avatar_url}
            avatarStyle={mention.member.avatar_style}
            avatarSeed={mention.member.avatar_seed}
            avatarBackgroundMode={mention.member.avatar_background_mode}
            avatarBackgroundColor={mention.member.avatar_background_color}
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
        </>
      )}
      {mention.team && (
        <>
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
        </>
      )}
    </span>,
    document.body,
  ) : null

  return (
    <span
      ref={ref}
      className="inline-flex"
      data-mention-type={mention.type}
      onMouseEnter={show}
      onMouseLeave={hide}
    >
      <span
        className={`cursor-pointer font-medium ${
          isTeam
            ? 'rounded-md border border-emerald-500/20 bg-emerald-500/10 px-1.5 py-0.5 text-emerald-700 dark:text-emerald-300'
            : 'rounded-md bg-blue-500/10 px-1 py-0.5 text-blue-600 dark:bg-blue-500/15 dark:text-blue-400'
        }`}
      >
        @{mention.member?.display_name ?? mention.team?.name ?? mention.handle}
      </span>
      {profileCard}
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
    const result: (string | MentionMatch | { __url: string })[] = []
    // Match @mentions or URLs (https/http)
    const regex = /(?:@([a-z0-9._-]+))|(https?:\/\/[^\s<>()]+(?:\([^\s<>()]*\))*[^\s<>().,;:!?"'\]])/gi
    let lastIndex = 0
    let match: RegExpExecArray | null

    while ((match = regex.exec(text)) !== null) {
      if (match.index > lastIndex) {
        result.push(text.slice(lastIndex, match.index))
      }
      if (match[1]) {
        result.push(resolveMention(match[1], membersByHandle, teamsByHandle))
      } else {
        result.push({ __url: match[0] })
      }
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
        ) : '__url' in part ? (
          <a
            key={i}
            href={part.__url}
            target="_blank"
            rel="noopener noreferrer"
            className="text-blue-600 underline break-all hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
          >
            {part.__url}
          </a>
        ) : (
          <MentionChip key={i} mention={part} />
        ),
      )}
    </span>
  )
}
