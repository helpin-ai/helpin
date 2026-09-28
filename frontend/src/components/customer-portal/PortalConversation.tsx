import { useEffect, useState } from 'react'
import Markdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { SupportAttachmentGallery } from '@/components/support/SupportAttachmentGallery'
import { BotIcon } from '@/lib/icons'
import type { CustomerPortalRequestDetail } from '@/lib/services/customerPortalService'
import { cn } from '@/lib/utils'
import { formatPortalAbsoluteTime, formatPortalRelativeTime } from './portalTime'

type PortalMessageData = CustomerPortalRequestDetail['messages'][number]

const REMARK_PLUGINS = [remarkGfm]

// Support and AI replies are Markdown. react-markdown renders no raw HTML
// and neutralises unsafe URLs; links open in a new tab.
const markdownComponents: Components = {
  a: ({ href, children }) => (
    <a href={href} target="_blank" rel="noopener noreferrer" className="break-words font-medium text-quiet-accent underline underline-offset-2 hover:text-quiet-accent-hover">
      {children}
    </a>
  ),
  p: ({ children }) => <p className="my-2 first:mt-0 last:mb-0">{children}</p>,
  ul: ({ children }) => <ul className="my-2 list-disc space-y-1 pl-5">{children}</ul>,
  ol: ({ children }) => <ol className="my-2 list-decimal space-y-1 pl-5">{children}</ol>,
  h1: ({ children }) => <p className="mb-1 mt-3 font-semibold first:mt-0">{children}</p>,
  h2: ({ children }) => <p className="mb-1 mt-3 font-semibold first:mt-0">{children}</p>,
  h3: ({ children }) => <p className="mb-1 mt-3 font-semibold first:mt-0">{children}</p>,
  strong: ({ children }) => <strong className="font-semibold">{children}</strong>,
  blockquote: ({ children }) => <blockquote className="my-2 border-l-2 border-quiet-divider-strong pl-3 text-quiet-text-secondary">{children}</blockquote>,
  code: ({ children }) => <code className="rounded bg-quiet-icon-well px-1 py-0.5 font-mono text-[12.5px]">{children}</code>,
  pre: ({ children }) => <pre className="my-2 overflow-x-auto rounded-md bg-quiet-icon-well p-3 font-mono text-[12.5px] [&_code]:bg-transparent [&_code]:p-0">{children}</pre>,
  table: ({ children }) => <div className="my-2 overflow-x-auto"><table className="w-full border-collapse text-left text-[13px]">{children}</table></div>,
  th: ({ children }) => <th className="border-b border-quiet-divider-strong py-1 pr-3 font-semibold">{children}</th>,
  td: ({ children }) => <td className="border-b border-quiet-divider-light py-1 pr-3">{children}</td>,
  img: ({ alt }) => <span>{alt}</span>,
}

/** PortalMessageContent renders support replies as Markdown and customer text as written. */
export function PortalMessageContent({ content, markdown }: { content: string; markdown: boolean }) {
  if (!markdown) return <p className="whitespace-pre-wrap break-words">{content}</p>
  return (
    <div className="break-words">
      <Markdown remarkPlugins={REMARK_PLUGINS} components={markdownComponents}>{content}</Markdown>
    </div>
  )
}

type GeneratedAvatar = NonNullable<PortalMessageData['sender_avatar_style']>

/**
 * useGeneratedAvatar draws a teammate's generated avatar. The avatar library
 * is loaded only when a thread needs it, keeping it out of the portal bundle.
 */
function useGeneratedAvatar(avatar: GeneratedAvatar | null | undefined) {
  const key = avatar?.style && avatar.seed ? JSON.stringify([avatar.style, avatar.seed, avatar.background_mode, avatar.background_color]) : ''
  const [drawn, setDrawn] = useState<{ key: string; src?: string }>({ key: '' })
  useEffect(() => {
    if (!key) return undefined
    let active = true
    const [style, seed, backgroundMode, backgroundColor] = JSON.parse(key) as (string | null | undefined)[]
    void import('@/lib/teamMemberAvatar').then(({ resolveTeamMemberAvatarSrc }) => {
      if (!active) return
      setDrawn({ key, src: resolveTeamMemberAvatarSrc({ avatarStyle: style, avatarSeed: seed, avatarBackgroundMode: backgroundMode, avatarBackgroundColor: backgroundColor }) })
    })
    return () => { active = false }
  }, [key])
  return drawn.key === key ? drawn.src : undefined
}

function SenderAvatar({ message }: { message: PortalMessageData }) {
  // The teammate's uploaded photo, else the generated avatar the app shows.
  const generatedSrc = useGeneratedAvatar(message.sender_type === 'ai' || message.sender_avatar ? null : message.sender_avatar_style)
  if (message.sender_type === 'ai') {
    return (
      <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-quiet-icon-well text-quiet-text-tertiary" aria-hidden="true">
        <BotIcon className="size-3.5" />
      </span>
    )
  }
  const src = message.sender_avatar || generatedSrc
  if (src) {
    return <img src={src} alt="" className="size-7 shrink-0 rounded-full object-cover" />
  }
  const initial = (message.sender_name || 'S').trim().charAt(0).toUpperCase()
  return (
    <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-quiet-icon-well text-[12px] font-semibold text-quiet-text-secondary" aria-hidden="true">
      {initial}
    </span>
  )
}

/**
 * PortalMessage shows one message. The customer's messages sit on the right;
 * support and AI replies sit on the left with an avatar, as in chat.
 */
export function PortalMessage({ message, supportName }: { message: PortalMessageData; supportName: string }) {
  const fromCustomer = message.sender_type === 'customer'
  const sender = fromCustomer ? 'You' : message.sender_name || supportName
  const time = (
    <time dateTime={message.created_at} title={formatPortalAbsoluteTime(message.created_at)}>
      {formatPortalRelativeTime(message.created_at)}
    </time>
  )
  const body = (
    <div className={cn('rounded-xl px-4 py-3 text-[14px] leading-[1.7] text-quiet-text-primary', fromCustomer ? 'bg-quiet-row-hover' : 'bg-quiet-hover')}>
      {message.content ? <PortalMessageContent content={message.content} markdown={!fromCustomer} /> : null}
      {message.attachments?.length ? (
        <SupportAttachmentGallery
          className={message.content ? 'mt-3' : ''}
          thumbnailSize="md"
          attachments={message.attachments.map((file) => ({ ...file, file_key: '' }))}
        />
      ) : null}
    </div>
  )

  if (fromCustomer) {
    return (
      <li className="flex justify-end py-2.5">
        <div className="min-w-0 max-w-[85%]">
          <p className="mb-1 text-right text-[12px] text-quiet-muted">
            <span className="font-medium text-quiet-text-secondary">{sender}</span> · {time}
          </p>
          {body}
        </div>
      </li>
    )
  }
  return (
    <li className="flex gap-2.5 py-2.5">
      <SenderAvatar message={message} />
      <div className="min-w-0 max-w-[85%] flex-1 sm:flex-none">
        <p className="mb-1 flex flex-wrap items-center gap-x-1.5 text-[12px] text-quiet-muted">
          <span className="font-medium text-quiet-text-secondary">{sender}</span>
          {message.sender_type === 'ai' ? (
            <span className="text-[10.5px] font-semibold uppercase tracking-[0.03em] text-quiet-text-tertiary">AI</span>
          ) : null}
          <span aria-hidden="true">·</span>
          {time}
        </p>
        {body}
      </div>
    </li>
  )
}

/** PortalActivityRow tells the customer someone is replying. */
export function PortalActivityRow({ label }: { label: string }) {
  return (
    <li className="flex items-center gap-2.5 py-2.5" role="status">
      <span className="flex size-7 shrink-0 items-center justify-center" aria-hidden="true">
        <span className="flex gap-0.5">
          {[0, 1, 2].map((dot) => (
            <span key={dot} className="size-1.5 rounded-full bg-quiet-muted motion-safe:animate-pulse" style={{ animationDelay: `${dot * 150}ms` }} />
          ))}
        </span>
      </span>
      <span className="text-[12.5px] text-quiet-text-tertiary">{label}</span>
    </li>
  )
}
