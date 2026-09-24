// @vitest-environment jsdom
import { act, type ComponentProps } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/stores/authStore'
import type { SupportMessage } from '@/lib/pmTypes'
import { TooltipProvider } from '@/components/ui/tooltip'
import { MessageBubble, sanitizeSupportShortcutSeed } from '../MessageBubble'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const LONG_PADDLE_URL = 'https://customer-portal.paddle.com/cpl_01jmc9m8bb4r0aqj6pwsfm3e28?action=update_subscription_payment_method&subscription_id=sub_01kjeeddmwt0qjg6snxcavej39&token=pga_eyJhbGciOiJFZERTQSIsImtpZCI6Imp3a18wMWhkazBtZDNzcHRtY3ZoYzR0dG0zZ2JoOSIsInR5cCI6IkpXVCJ9.eyJpZCI6InBnYV8wMWtwMnJtY2dnOHFtbmUyeHg4MWM5c3RldiIsInNlbGxlci1pZCI6IjIxNzkxNyIsInR5cGUiOiJzdGFuZGFyZCIsInZlcnNpb24iOiIxIiwidXNhZ2UiOiJjdXN0b21lci1wb3J0YWwtdXJsIiwic2NvcGUiOiJjdXN0b21lci5hZGp1c3RtZW50LnJlYWQgY3VzdG9tZXIuY2hlY2tvdXQuY3JlYXRlIGN1c3RvbWVyLmNoZWNrb3V0LnJlYWQgY3VzdG9tZXIuY3VzdG9tZXIucmVhZCBjdXN0b21lci5jdXN0b21lci51cGRhdGUgY3VzdG9tZXIuY3VzdG9tZXItYWRkcmVzcy5yZWFkIGN1c3RvbWVyLmN1c3RvbWVyLWFkZHJlc3MudXBkYXRlIGN1c3RvbWVyLmN1c3RvbWVyLWJ1c2luZXNzLnJlYWQgY3VzdG9tZXIuY3VzdG9tZXItYnVzaW5lc3MuY3JlYXRlIGN1c3RvbWVyLmN1c3RvbWVyLWJ1c2luZXNzLnVwZGF0ZSBjdXN0b21lci5jdXN0b21lci1wYXltZW50LW1ldGhvZC5yZWFkIGN1c3RvbWVyLmN1c3RvbWVyLXBheW1lbnQtbWV0aG9kLmRlbGV0ZSBjdXN0b21lci5pbnZvaWNlLnJlYWQgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLWNhbmNlbC5jcmVhdGUgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLWNvbnNlbnQtcmVxdWlyZW1lbnQtZ3JhbnQuY3JlYXRlIGN1c3RvbWVyLnN1YnNjcmlwdGlvbi1jb25zZW50LXJlcXVpcmVtZW50LnJlYWQgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLnJlYWQgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLnVwZGF0ZSBjdXN0b21lci50cmFuc2FjdGlvbi5jcmVhdGUgY3VzdG9tZXIudHJhbnNhY3Rpb24ucmVhZCBjdXN0b21lci50cmFuc2FjdGlvbi51cGRhdGUgY3VzdG9tZXIudHJhbnNhY3Rpb24ub3JpZ2luLnJlYWQiLCJpc3MiOiJndWVzdGFjY2Vzcy1zZXJ2aWNlIiwic3ViIjoiY3RtXzAxa2plZTRxdGpkeTBuMTZ4cmZ4cXllY2NtIiwiZXhwIjoxNzc2MTQ4MzE5LCJpYXQiOjE3NzYwNjE5MTl9.u1Lm-pF347MaDE92MtneXBR6a4KxXiobrEBXjXjVm5Jk49qqGHKSYpUswzVyZy9BU0kl26tvkonj3gjMVquzAA'

function findButtonByText(container: HTMLElement, text: string) {
  return Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes(text)) ?? null
}

function renderBubble(
  message: SupportMessage,
  receiptStatus?: 'sending_email' | 'delivered' | 'sent_email' | 'delivered_email' | 'read' | 'read_email' | 'sent_outside_helpin' | null,
  extraProps: Partial<ComponentProps<typeof MessageBubble>> = {},
) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  const queryClient = new QueryClient()

  act(() => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <MessageBubble message={message} receiptStatus={receiptStatus} {...extraProps} />
        </TooltipProvider>
      </QueryClientProvider>,
    )
  })

  return {
    container,
    queryClient,
    cleanup: () => {
      act(() => {
        root.unmount()
      })
      container.remove()
      queryClient.clear()
    },
  }
}

describe('MessageBubble', () => {
  it('keeps translation controls and timestamp together inside the bubble', () => {
    const rendered = renderBubble({ id: 'translated', workspace_id: 'ws', conversation_id: 'conv', sender_type: 'customer', message_type: 'reply', is_internal: false, content: 'Hello', created_at: '2026-09-24T10:00:00Z', updated_at: '2026-09-24T10:00:00Z' }, undefined, { translationFooter: <button>Show original</button> });
    try {
      const footer = rendered.container.querySelector('[data-slot="support-message-footer"]');
      expect(footer?.querySelector('button')?.textContent).toBe('Show original');
      expect(footer?.querySelector('time')?.dateTime).toBe('2026-09-24T10:00:00Z');
      expect(footer?.closest('[data-slot="support-message-bubble-frame"]')).not.toBeNull();
    } finally { rendered.cleanup(); }
  });

  it.each(['ai_paused', 'ai_returned'] as const)('renders %s with the recorded actor', (event) => {
    const rendered = renderBubble({
      id: 'activity', workspace_id: 'ws', conversation_id: 'conv', sender_type: 'user',
      sender_user_id: 'arooj', sender_display_name: 'Arooj Bukhari', message_type: 'system', system_event_type: event, is_internal: true,
      content: event === 'ai_returned' ? 'Returned to AI. AI will respond to the next customer message.' : 'AI paused.',
      created_at: '2026-09-18T10:38:14Z', updated_at: '2026-09-18T10:38:14Z',
    })
    try {
      expect(rendered.container.textContent).toContain(event === 'ai_paused' ? 'Arooj paused AI.' : 'Arooj returned the conversation to AI.')
      expect(rendered.container.querySelector('[data-support-ai-activity]')).not.toBeNull()
      expect(rendered.container.textContent).not.toContain('left a private note')
    } finally { rendered.cleanup() }
  })

  it('keeps a legacy pause note accessible without displaying another handoff card', () => {
    const rendered = renderBubble({
      id: 'pause', workspace_id: 'ws', conversation_id: 'conv', sender_type: 'agent',
      sender_user_id: 'former-member', sender_display_name: 'AI control', message_type: 'note', is_internal: true,
      content: 'AI handoff with historical context',
      metadata: JSON.stringify({ ai_handoff_brief: true, agent_authored: false, reason: 'paused_by_teammate' }),
      created_at: '2026-09-18T10:38:14Z', updated_at: '2026-09-18T10:38:14Z',
    })
    try {
      expect(rendered.container.textContent).toContain('A teammate paused AI.')
      expect(rendered.container.querySelector('[data-support-ai-handoff]')).toBeNull()
      const details = rendered.container.querySelector('details')
      expect(details?.open).toBe(false)
      expect(details?.querySelector('summary')?.textContent).toBe('View original note')
      expect(details?.textContent).toContain('historical context')
      act(() => { details!.open = true })
      expect(details?.open).toBe(true)
    } finally { rendered.cleanup() }
  })

  it('does not mistake ordinary private notes for AI activity', () => {
    const rendered = renderBubble({
      id: 'ordinary', workspace_id: 'ws', conversation_id: 'conv', sender_type: 'user',
      sender_display_name: 'Arooj Bukhari', message_type: 'note', is_internal: true,
      content: 'Returned to AI. AI will respond to the next customer message.', metadata: '{invalid',
      created_at: '2026-09-18T10:38:14Z', updated_at: '2026-09-18T10:38:14Z',
    })
    try {
      expect(rendered.container.textContent).toContain('left a private note')
      expect(rendered.container.querySelector('[data-support-ai-activity]')).toBeNull()
    } finally { rendered.cleanup() }
  })

  it('renders a legacy return note as an attributed activity', () => {
    const rendered = renderBubble({
      id: 'return', workspace_id: 'ws', conversation_id: 'conv', sender_type: 'agent',
      sender_user_id: 'arooj', sender_display_name: 'AI control', message_type: 'note', is_internal: true,
      content: 'Returned to AI. AI will respond to the next customer message.', metadata: '{}',
      created_at: '2026-09-18T10:38:14Z', updated_at: '2026-09-18T10:38:14Z',
    }, null, { teammateDisplayName: 'Arooj Bukhari' })
    try {
      expect(rendered.container.textContent).toContain('Arooj returned the conversation to AI')
      expect(rendered.container.textContent).not.toContain('left a private note')
    } finally { rendered.cleanup() }
  })

  it('shows an AI handoff with expandable investigation details', () => {
    const rendered = renderBubble({
      id: 'handoff', workspace_id: 'ws', conversation_id: 'conv', sender_type: 'agent',
      sender_display_name: 'AI control', message_type: 'note', is_internal: true,
      content: 'AI handoff\n\nIssue\nWorkspace switch fails\n\nAlready tried / suggested\n- Checked related tickets\n\nStill unresolved\n- Verify call sites\n\nReason for handoff\ncannot answer',
      metadata: JSON.stringify({ ai_handoff_brief: true, agent_authored: true }),
      created_at: '2026-09-18T10:32:14Z', updated_at: '2026-09-18T10:32:14Z',
    })
    try {
      expect(rendered.container.textContent).not.toContain('left a private note')
      expect(rendered.container.textContent).toContain('Workspace switch fails')
      const details = rendered.container.querySelector('details')
      expect(details).not.toBeNull()
      expect(details?.open).toBe(false)
      expect(details?.querySelector('summary')?.textContent).toBe('View details')
      expect(details?.textContent).toContain('Checked related tickets')
      expect(details?.textContent).not.toContain('Workspace switch fails')
      expect(rendered.container.textContent?.match(/Checked related tickets/g)).toHaveLength(1)
    } finally { rendered.cleanup() }
  })

  it.each([true, false])('shows saved visitor feedback below the AI bubble (%s)', async (helpful) => {
    const message: SupportMessage = {
      id: 'feedback-answer', workspace_id: 'ws-1', conversation_id: 'conv-1',
      sender_type: 'ai', content: 'A useful answer', message_type: 'reply', is_internal: false,
      metadata: JSON.stringify({ visitor_feedback: { helpful, submitted_at: '2026-09-13T12:00:00Z' } }),
      created_at: '2026-09-13T11:59:00Z', updated_at: '2026-09-13T12:00:00Z',
    }
    const rendered = renderBubble(message)
    try {
      const row = rendered.container.querySelector('[data-slot="support-answer-feedback"]')
      const frame = rendered.container.querySelector('[data-slot="support-message-bubble-frame"]')
      const label = helpful ? 'Visitor marked helpful' : 'Visitor marked unhelpful'
      const badge = row?.querySelector('[role="img"]')
      expect(badge?.getAttribute('aria-label')).toBe(label)
      expect(frame?.contains(row)).toBe(false)
      expect(row?.className).toContain('justify-start')
      await act(async () => { badge!.dispatchEvent(new FocusEvent('focusin', { bubbles: true })) })
      expect(document.querySelector('[role="tooltip"]')?.textContent).toBe(label)
    } finally { rendered.cleanup() }
  })

  it.each([
    { sender: 'customer', via: 'email', source: 'widget', expected: 'via Email' },
    { sender: 'customer', via: 'widget', source: 'email', expected: 'via Chat' },
    { sender: 'user', via: 'email', source: 'widget', expected: 'via Email' },
    { sender: 'user', via: 'email', source: 'widget', mode: 'chat_and_email', expected: 'via Chat + email' },
    { sender: 'user', via: 'email', source: 'email', internal: true, expected: null },
  ] as const)('uses the message channel in the time tooltip: $sender/$via/$source/$expected', async ({ sender, via, source, expected, ...options }) => {
    const message: SupportMessage = {
      id: 'channel-message', workspace_id: 'ws-1', conversation_id: 'conv-1',
      sender_type: sender, content: 'Message channel', message_type: 'reply',
      is_internal: 'internal' in options && options.internal,
      via_channel: via,
      metadata: 'mode' in options ? JSON.stringify({ delivery_mode: options.mode }) : undefined,
      created_at: '2026-09-13T09:04:00Z', updated_at: '2026-09-13T09:04:00Z',
    }
    const rendered = renderBubble(message, undefined, { source })
    try {
      const trigger = rendered.container.querySelector('time')?.closest('[data-slot="tooltip-trigger"]')
      expect(trigger).toBeTruthy()
      await act(async () => { trigger!.dispatchEvent(new FocusEvent('focusin', { bubbles: true })) })
      const tooltip = document.querySelector('[role="tooltip"]')
      expect(tooltip).toBeTruthy()
      if (expected) expect(tooltip?.textContent).toContain(expected)
      else expect(tooltip?.textContent).not.toContain('via ')
    } finally { rendered.cleanup() }
  })

  it.each([
    ['customer', false],
    ['user', false],
    ['ai', false],
    ['agent', false],
    ['user', true],
  ] as const)('shows a clock time on grouped %s messages (note: %s)', (senderType, isInternal) => {
    const createdAt = '2026-09-13T09:04:00.000Z'
    const message: SupportMessage = {
      id: 'timed-message', workspace_id: 'ws-1', conversation_id: 'conv-1',
      sender_type: senderType, content: 'Yes', message_type: 'reply',
      is_internal: isInternal, created_at: createdAt, updated_at: createdAt,
    }
    const rendered = renderBubble(message, null, { isConsecutive: true, isLastInGroup: false })
    const time = rendered.container.querySelector('time')
    expect(time?.getAttribute('datetime')).toBe(createdAt)
    expect(time?.textContent).toBe(new Date(createdAt).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }))
    expect(time?.getAttribute('title')).toBeTruthy()
    rendered.cleanup()
  })

  it('shows one timestamp for an image-only message', () => {
    const message: SupportMessage = {
      id: 'timed-image', workspace_id: 'ws-1', conversation_id: 'conv-1',
      sender_type: 'customer', content: '', message_type: 'reply', is_internal: false,
      created_at: '2026-09-13T09:04:00.000Z', updated_at: '2026-09-13T09:04:00.000Z',
      attachments: [{ id: 'image-1', file_name: 'photo.png', file_type: 'image/png', file_size: 100, file_key: 'photo.png' }],
    }
    const rendered = renderBubble(message)
    expect(rendered.container.querySelectorAll('time')).toHaveLength(1)
    rendered.cleanup()
  })

  beforeEach(() => {
    if (!globalThis.ResizeObserver) {
      globalThis.ResizeObserver = class ResizeObserver {
        observe() {}
        unobserve() {}
        disconnect() {}
      }
    }
    useAuthStore.setState({
      user: {
        id: 'viewer-1',
        email: 'viewer@example.com',
        full_name: 'Viewer',
        organization_id: 'org-1',
        created_at: '2026-04-01T00:00:00Z',
        updated_at: '2026-04-01T00:00:00Z',
      },
      loading: false,
      serverUnreachable: false,
    })
  })

  it.each([
    { senderID: 'teammate-2', expectedName: 'Teammate' },
    { senderID: undefined, expectedName: 'Teammate' },
    { senderID: 'viewer-1', expectedName: 'Viewer' },
  ])('only falls back to the viewer name for their own message ($senderID)', ({ senderID, expectedName }) => {
    const rendered = renderBubble({
      id: 'reply', workspace_id: 'ws-1', conversation_id: 'conv-1',
      sender_type: 'user', sender_user_id: senderID,
      sender_avatar_url: '/sender.png', content: 'Hello', is_internal: false,
      created_at: '2026-09-15T09:00:00Z', updated_at: '2026-09-15T09:00:00Z',
    })
    try {
      expect(rendered.container.querySelector('img[src="/sender.png"]')?.getAttribute('alt')).toBe(expectedName)
    } finally { rendered.cleanup() }
  })

  it('removes markdown hard-break escapes when seeding shortcut content', () => {
    expect(sanitizeSupportShortcutSeed('Hi Caleb,\\\n\\\nThank you for reaching out.')).toBe(
      'Hi Caleb,\n\nThank you for reaching out.',
    )
  })

  afterEach(() => {
    document.body.innerHTML = ''
    useAuthStore.setState({ user: null, loading: false, serverUnreachable: false })
  })

  it('uses source-specific icon avatars for automated routing events', () => {
    const ruleMessage: SupportMessage = {
      id: 'msg-rule-route',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'ai',
      sender_display_name: 'Routing',
      content: "Routing rule moved to inbox 'Billing'.",
      message_type: 'system',
      system_event_type: 'triage_routed',
      is_internal: true,
      created_at: '2026-06-12T09:00:00.000Z',
      updated_at: '2026-06-12T09:00:00.000Z',
    }
    const aiMessage: SupportMessage = {
      ...ruleMessage,
      id: 'msg-ai-route',
      sender_display_name: 'Helpin AI',
      content: "AI routing moved to inbox 'Billing'.",
    }

    const renderedRule = renderBubble(ruleMessage)
    expect(renderedRule.container.querySelector('[aria-label="Routing rule"]')).toBeTruthy()
    expect(renderedRule.container.textContent).not.toContain('RRouting rule')
    renderedRule.cleanup()

    const renderedAI = renderBubble(aiMessage)
    expect(renderedAI.container.querySelector('[aria-label="AI routing"]')).toBeTruthy()
    expect(renderedAI.container.textContent).not.toContain('HAI routing')
    renderedAI.cleanup()
  })

  it('renders resolved system messages with a green check treatment', () => {
    const message: SupportMessage = {
      id: 'msg-resolved',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Sarah Khan',
      content: 'Sarah resolved this conversation.',
      message_type: 'system',
      system_event_type: 'resolved',
      is_internal: true,
      created_at: '2026-06-12T09:00:00.000Z',
      updated_at: '2026-06-12T09:00:00.000Z',
    }

    const rendered = renderBubble(message)
    const resolvedIcon = rendered.container.querySelector('[aria-label="Resolved"]')
    expect(resolvedIcon).toBeTruthy()
    expect(resolvedIcon?.className).toContain('text-emerald-600')
    const actorAvatar = rendered.container.querySelector('img[alt="Sarah Khan"], [aria-label="Sarah Khan"]')
    expect(actorAvatar).toBeTruthy()
    expect(resolvedIcon?.compareDocumentPosition(actorAvatar as Node) ?? 0).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
    expect(rendered.container.querySelector('.px-3.py-1.text-xs')).toBeTruthy()
    expect(rendered.container.querySelector('.text-sm')?.textContent).not.toBe('Sarah resolved this conversation.')
    expect(rendered.container.textContent).toContain('Sarah resolved this conversation.')
    rendered.cleanup()
  })

  it('renders reopened system messages like neutral routing timeline events', () => {
    const message: SupportMessage = {
      id: 'msg-reopened',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Sarah Khan',
      content: 'Sarah reopened this conversation.',
      message_type: 'system',
      system_event_type: 'reopened',
      is_internal: true,
      created_at: '2026-06-12T09:00:00.000Z',
      updated_at: '2026-06-12T09:00:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.querySelector('.px-3.py-1.text-xs')).toBeTruthy()
    expect(rendered.container.querySelector('[aria-label="Reopened"]')).toBeNull()
    expect(rendered.container.textContent).toContain('Sarah reopened this conversation.')
    rendered.cleanup()
  })

  it('preserves the assignee in assignment system event content', () => {
    const message: SupportMessage = {
      id: 'msg-assigned-legacy',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_display_name: 'Sarah Khan',
      content: 'Sarah assigned this conversation to Adeel.',
      message_type: 'system',
      system_event_type: 'assigned',
      is_internal: true,
      created_at: '2026-06-12T09:00:00.000Z',
      updated_at: '2026-06-12T09:00:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.textContent).toContain('Sarah assigned this conversation to Adeel.')
    expect(rendered.container.querySelector('strong')?.textContent).toBe('Adeel')
    rendered.cleanup()
  })

  it('bolds important values in support audit system events', () => {
    const messages: SupportMessage[] = [
      {
        id: 'msg-recipient-updated',
        workspace_id: 'ws-1',
        conversation_id: 'conv-1',
        sender_type: 'user',
        sender_display_name: 'Sarah Khan',
        content: 'Sarah made jane@example.com the primary recipient. Sarah added teammate@example.com to Cc.',
        message_type: 'system',
        system_event_type: 'email_recipients_updated',
        is_internal: true,
        created_at: '2026-06-12T09:00:00.000Z',
        updated_at: '2026-06-12T09:00:00.000Z',
      },
      {
        id: 'msg-tag-added',
        workspace_id: 'ws-1',
        conversation_id: 'conv-1',
        sender_type: 'user',
        sender_display_name: 'Sarah Khan',
        content: 'Sarah added tag Billing.',
        message_type: 'system',
        system_event_type: 'tag_added',
        is_internal: true,
        created_at: '2026-06-12T09:00:00.000Z',
        updated_at: '2026-06-12T09:00:00.000Z',
      },
      {
        id: 'msg-task-created',
        workspace_id: 'ws-1',
        conversation_id: 'conv-1',
        sender_type: 'user',
        sender_display_name: 'Sarah Khan',
        content: 'Sarah created task #ENG-123: Fix billing webhook.',
        message_type: 'system',
        system_event_type: 'task_created',
        is_internal: true,
        created_at: '2026-06-12T09:00:00.000Z',
        updated_at: '2026-06-12T09:00:00.000Z',
      },
    ]

    const rendered = messages.map((message) => renderBubble(message, undefined, {
      workspaceSlug: 'acme',
      linkedTaskId: 'task-123',
    }))
    expect(rendered[0].container.querySelectorAll('strong')[0]?.textContent).toBe('jane@example.com')
    expect(rendered[0].container.querySelectorAll('strong')[1]?.textContent).toBe('teammate@example.com')
    expect(rendered[1].container.querySelector('strong')?.textContent).toBe('Billing')
    expect(rendered[2].container.querySelectorAll('strong')[0]?.textContent).toBe('#ENG-123')
    expect(rendered[2].container.querySelectorAll('strong')[1]?.textContent).toBe('Fix billing webhook')
    expect(rendered[2].container.querySelector('a')?.getAttribute('href')).toBe('/w/acme/pm/tasks/task-123')
    expect(rendered[2].container.querySelector('[data-support-system-callout]')?.className).toContain('max-w-[70%]')
    expect(rendered[2].container.querySelector('[data-task-created-event]')?.className).not.toContain('mx-auto')
    expect(rendered[2].container.querySelector('[data-task-created-prefix]')?.className).toContain('mr-1')
    expect(rendered[2].container.querySelector('[data-task-created-link]')?.className).toContain('hover:bg-muted')
    expect(rendered[2].container.querySelector('[data-task-created-title]')?.className).toContain('truncate')
    rendered.forEach((entry) => entry.cleanup())
  })

  it('uses canonical copy for historical AI escalation system events', () => {
    const message: SupportMessage = {
      id: 'msg-ai-escalated-legacy',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'agent',
      sender_display_name: 'Helpin AI',
      content: 'Let me connect you with a team member who can help.',
      message_type: 'system',
      system_event_type: 'ai_escalated',
      is_internal: true,
      created_at: '2026-06-12T09:00:00.000Z',
      updated_at: '2026-06-12T09:00:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.textContent).toContain('AI escalated to a human')
    expect(rendered.container.textContent).not.toContain('Let me connect you with a team member who can help.')
    rendered.cleanup()
  })

  it('renders a long billing link message with wrapping-safe anchors and link previews', () => {
    const message: SupportMessage = {
      id: 'msg-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Daniyal Ali Sajid',
      content: `Hi Patrick,\n\nUse the secure link below:\n\n${LONG_PADDLE_URL}\n\nLet us know if you run into any issues!`,
      message_type: 'reply',
      is_internal: false,
      via_channel: 'widget',
      metadata: JSON.stringify({
        link_previews: [
          {
            url: LONG_PADDLE_URL,
            host: 'customer-portal.paddle.com',
            title: 'Customer Portal',
            description: 'Manage billing details and subscription settings.',
            image_url: 'https://customer-portal.paddle.com/preview.png',
          },
        ],
      }),
      created_at: '2026-04-13T06:40:07.886147Z',
      updated_at: '2026-04-13T06:42:09.73606Z',
    }

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = new QueryClient()

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <TooltipProvider>
            <MessageBubble message={message} />
          </TooltipProvider>
        </QueryClientProvider>,
      )
    })

    expect(container.textContent).toContain('Customer Portal')
    expect(container.textContent).not.toContain('Manage billing details and subscription settings.')
    expect(container.querySelector('img[src="https://customer-portal.paddle.com/preview.png"]')).toBeNull()

    const bubble = container.querySelector('[data-slot="support-message-bubble"]')
    expect(bubble?.className).toContain('min-w-0')

    const rawLink = Array.from(container.querySelectorAll('a')).find((anchor) => anchor.getAttribute('href') === LONG_PADDLE_URL)
    expect(rawLink).toBeTruthy()
    expect(rawLink?.className).toContain('[overflow-wrap:anywhere]')

    act(() => {
      root.unmount()
    })
    container.remove()
    queryClient.clear()
  })

  it('marks HTTP links and previews as not secure without blocking navigation', () => {
    const message: SupportMessage = {
      id: 'msg-http', workspace_id: 'ws-1', conversation_id: 'conv-1', sender_type: 'customer',
      content: 'Open [the page](http://example.com/path)', message_type: 'reply', is_internal: false,
      metadata: JSON.stringify({ link_previews: [{ url: 'http://example.com/path', host: 'example.com', title: 'Example' }] }),
      created_at: '2026-08-18T12:00:00Z', updated_at: '2026-08-18T12:00:00Z',
    }
    const rendered = renderBubble(message)

    expect(rendered.container.querySelector('[aria-label="Not secure"]')).toBeTruthy()
    expect(rendered.container.textContent).toContain('Not secure')
    expect(rendered.container.querySelector('a[href="http://example.com/path"]')?.getAttribute('rel')).toBe('noopener noreferrer')
    rendered.cleanup()
  })

  it('intercepts confirmed malicious links until risk is acknowledged', () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null)
    const message: SupportMessage = {
      id: 'msg-malicious', workspace_id: 'ws-1', conversation_id: 'conv-1', sender_type: 'customer',
      content: 'Open [the invoice](https://bad.example/invoice)', message_type: 'reply', is_internal: false,
      metadata: JSON.stringify({
        link_security: [{
          url: 'https://bad.example/invoice', status: 'malicious', threat_types: ['SOCIAL_ENGINEERING'],
          checked_at: '2026-08-18T12:00:00Z', expires_at: '2099-08-18T12:30:00Z',
        }],
      }),
      created_at: '2026-08-18T12:00:00Z', updated_at: '2026-08-18T12:00:00Z',
    }
    const rendered = renderBubble(message)
    const link = rendered.container.querySelector('a[href="https://bad.example/invoice"]') as HTMLAnchorElement

    act(() => link.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true })))
    expect(document.body.textContent).toContain('Potentially harmful link')
    expect(document.body.textContent).toContain('Phishing or deceptive site')
    const continueButton = findButtonByText(document.body, 'Open link') as HTMLButtonElement
    expect(continueButton.disabled).toBe(true)

    const checkbox = document.body.querySelector('[data-slot="checkbox"]') as HTMLElement
    act(() => checkbox.click())
    expect(continueButton.disabled).toBe(false)
    act(() => continueButton.click())
    expect(open).toHaveBeenCalledWith('https://bad.example/invoice', '_blank', 'noopener,noreferrer')

    open.mockRestore()
    rendered.cleanup()
  })

  it('constrains email iframe content to the message bubble width', () => {
    const message: SupportMessage = {
      id: 'msg-email-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Customer',
      content: 'Email body',
      html_body: '<div style="white-space: nowrap">Hello from a long email body that should not push into the details sidebar.</div>',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = new QueryClient()

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <TooltipProvider>
            <MessageBubble message={message} />
          </TooltipProvider>
        </QueryClientProvider>,
      )
    })

    const bubble = container.querySelector('[data-slot="support-message-bubble"]')
    expect(bubble?.className).toContain('w-[min(92%,64rem)]')
    expect(bubble?.className).toContain('max-w-[calc(100%-2.25rem)]')

    const iframe = container.querySelector('iframe[title="Email body"]') as HTMLIFrameElement | null
    expect(iframe).toBeTruthy()
    expect(iframe?.style.maxWidth).toBe('100%')
    expect(iframe?.style.minWidth).toBe('0px')
    expect(iframe?.style.minWidth).not.toBe('602px')
    expect(iframe?.getAttribute('srcdoc')).toContain('overflow-wrap: anywhere !important')

    act(() => {
      root.unmount()
    })
    container.remove()
    queryClient.clear()
  })

  it('uses the backend visible email projection instead of full fallback content', () => {
    const message: SupportMessage = {
      id: 'msg-email-projection-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Customer',
      content: 'Fresh reply\n\nOn Tuesday someone wrote:\nOld quoted body',
      email_visible_text: 'Fresh reply',
      email_quoted_text: 'On Tuesday someone wrote:\nOld quoted body',
      email_has_quoted_content: true,
      email_projection_confidence: 'high',
      email_projection_version: 1,
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.textContent).toContain('Fresh reply')
    expect(rendered.container.textContent).not.toContain('Old quoted body')
    rendered.cleanup()
  })

  it('defaults rich quoted email HTML to collapsed even with attribution metadata', () => {
    const message: SupportMessage = {
      id: 'msg-email-projection-2',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Jane Customer',
      content: 'Fresh reply',
      html_body: '<p>Fresh reply</p><div data-helpin-quote="true">Old quoted body</div>',
      email_visible_text: 'Fresh reply',
      email_quoted_text: 'Old quoted body',
      email_has_quoted_content: true,
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      metadata: JSON.stringify({
        forwarded_by_email: 'founder@company.com',
        original_sender_email: 'jane@customer.example',
      }),
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const rendered = renderBubble(message)
    const iframe = rendered.container.querySelector('iframe[title="Email body"]')
    expect(iframe?.getAttribute('data-collapsed-by-default')).toBe('true')
    rendered.cleanup()
  })

  it('renders compact forwarded attribution in the email badge', () => {
    const message: SupportMessage = {
      id: 'msg-forwarded-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Jane Customer',
      content: 'I need help with my invoice.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      metadata: JSON.stringify({
        forwarded_by_email: 'founder@company.com',
        forwarded_by_name: 'Founder',
        original_sender_email: 'jane@customer.example',
        original_sender_name: 'Jane Customer',
        sender_attribution_confidence_level: 'high',
      }),
      created_at: '2026-06-02T10:14:00.000Z',
      updated_at: '2026-06-02T10:14:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.textContent).toContain('Forwarded by Founder')
    expect(rendered.container.textContent).not.toContain('View details')
    expect(rendered.container.textContent).not.toContain('Received via email')
    expect(rendered.container.textContent).not.toContain('originally from')
    expect(rendered.container.textContent).not.toContain('jane@customer.example')
    expect(findButtonByText(rendered.container, 'Forwarded by Founder')).toBeTruthy()
    rendered.cleanup()
  })

  it('shows from email in the inbound email badge when present', () => {
    const message: SupportMessage = {
      id: 'msg-reply-to-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Taylor Visitor',
      content: 'Can someone contact me?',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      email_from: 'Acme Contact Form <website@acme.com>',
      email_reply_to: 'Taylor Visitor <taylor.visitor@example.com>',
      created_at: '2026-06-02T10:14:00.000Z',
      updated_at: '2026-06-02T10:14:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.textContent).toContain('Received by email from website@acme.com')
    expect(rendered.container.textContent).not.toContain('Received by email from taylor.visitor@example.com')
    expect(rendered.container.textContent).not.toContain('Received via email')
    expect(findButtonByText(rendered.container, 'Received by email from website@acme.com')).toBeTruthy()
    rendered.cleanup()
  })

  it('shows a generic inbound email badge when from and reply-to match', () => {
    const message: SupportMessage = {
      id: 'msg-reply-to-same-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Taylor Visitor',
      content: 'Following up here.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      email_from: 'Taylor Visitor <taylor.visitor@example.com>',
      email_reply_to: 'taylor.visitor@example.com',
      created_at: '2026-06-02T10:14:00.000Z',
      updated_at: '2026-06-02T10:14:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.textContent).toContain('Received by email')
    expect(rendered.container.textContent).not.toContain('Received by email from taylor.visitor@example.com')
    expect(findButtonByText(rendered.container, 'Received by email')).toBeTruthy()
    rendered.cleanup()
  })

  it('shows a generic inbound email badge when from matches the conversation customer email', () => {
    const message: SupportMessage = {
      id: 'msg-customer-email-same-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Taylor Visitor',
      content: 'Following up here.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      email_from: 'Taylor Visitor <taylor.visitor@example.com>',
      email_reply_to: 'support-thread+123@example.com',
      created_at: '2026-06-02T10:14:00.000Z',
      updated_at: '2026-06-02T10:14:00.000Z',
    }

    const rendered = renderBubble(message, undefined, { customerEmail: 'taylor.visitor@example.com' })
    expect(rendered.container.textContent).toContain('Received by email')
    expect(rendered.container.textContent).not.toContain('Received by email from taylor.visitor@example.com')
    expect(findButtonByText(rendered.container, 'Received by email')).toBeTruthy()
    rendered.cleanup()
  })

  it('shows the forwarded customer body when the email has no note above the forwarded header', () => {
    const message: SupportMessage = {
      id: 'msg-forwarded-empty-note-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Jason Smith',
      content: `---------- Forwarded message ---------
From: Jason Smith <jason@the-web-dev.com>
Date: Sun, May 31, 2026 at 2:38 PM
Subject: Data Export
To: Waqar from Usermaven <waqar@usermaven.com>

Can I export my data?`,
      html_body: '<div data-helpin-quote="true">Can I export my data?</div>',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      metadata: JSON.stringify({
        forwarded_by_email: 'waqar@usermaven.com',
        forwarded_by_name: 'Waqar Azeem',
        original_sender_email: 'jason@the-web-dev.com',
        original_sender_name: 'Jason Smith',
        sender_attribution_confidence_level: 'high',
      }),
      created_at: '2026-06-02T14:40:00.000Z',
      updated_at: '2026-06-02T14:40:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.textContent).toContain('Can I export my data?')
    expect(rendered.container.textContent).not.toContain('Forwarded message')
    expect(rendered.container.textContent).not.toContain('From: Jason Smith')
    expect(rendered.container.textContent).toContain('Forwarded by Waqar Azeem')
    rendered.cleanup()
  })

  it('renders outbound fallback email delivery receipt states', () => {
    const message: SupportMessage = {
      id: 'msg-email-status-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Agent',
      content: 'Following up here.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      email_notified_at: '2026-04-24T12:20:00.000Z',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:20:00.000Z',
    }

    const sent = renderBubble(message, 'sent_email')
    expect(sent.container.textContent).toContain('Sent via email')
    expect(sent.container.textContent).not.toContain('View details')
    expect(sent.container.textContent?.match(/Sent via email/g)).toHaveLength(1)
    expect(findButtonByText(sent.container, 'Sent via email')).toBeTruthy()
    sent.cleanup()

    const sending = renderBubble({ ...message, id: 'msg-email-status-sending', email_notified_at: undefined }, 'sending_email')
    expect(sending.container.textContent).toContain('Sending email')
    expect(sending.container.textContent).not.toContain('Delivered')
    sending.cleanup()

    const delivered = renderBubble({ ...message, id: 'msg-email-status-delivered', email_delivery_status: 'delivered' }, 'delivered_email')
    expect(delivered.container.textContent).toContain('Delivered via email')
    delivered.cleanup()

    const read = renderBubble({ ...message, id: 'msg-email-status-2', email_read_at: '2026-04-24T12:22:00.000Z' }, 'read_email')
    expect(read.container.textContent).toContain('Read via email')
    read.cleanup()
  })

  it('opens the original email from message info for an emailed chat reply', () => {
    const message: SupportMessage = {
      id: 'msg-email-info-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Agent',
      content: 'Following up here.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'widget',
      email_delivery_status: 'delivered',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:20:00.000Z',
    }

    const rendered = renderBubble(message, 'delivered_email')
    rendered.queryClient.setQueryData(['support', 'ws-1', 'conversations', 'conv-1', 'messages', message.id, 'info'], {
      id: message.id, sent_at: message.created_at, sender: { name: 'Agent', type: 'user' },
      from: 'support@example.com', origin: 'email', type: 'text', email_direction: 'outbound',
      email_delivery_status_label: 'Delivered via email',
    })
    rendered.queryClient.setQueryData(['support', 'ws-1', 'messages', message.id, 'email'], {
      id: 'email-log', message_id: message.id, direction: 'outbound', subject: 'Original email subject',
      from_email: 'support@example.com', to_email: 'customer@example.com', stripped_text: message.content,
      created_at: message.created_at,
    })
    const marker = findButtonByText(rendered.container, 'Delivered via email')
    expect(marker).toBeTruthy()

    act(() => {
      marker?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(document.body.textContent).toContain('Message info')
    const viewEmail = findButtonByText(document.body, 'View original email')
    expect(viewEmail).toBeTruthy()
    act(() => viewEmail!.click())
    expect(document.body.textContent).toContain('Original email subject')
    expect(document.body.textContent).toContain('customer@example.com')
    expect(document.body.textContent).not.toContain('Message info')
    rendered.cleanup()
  })

  it('labels copied teammate email replies as sent outside Helpin', () => {
    const message: SupportMessage = {
      id: 'msg-external-email-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Agent',
      content: 'Sent directly from Gmail.',
      message_type: 'reply',
      is_internal: false,
      metadata: '{"external_email_reply":true,"external_email_capture":"support_email_copy"}',
      via_channel: 'email',
      created_at: '2026-09-03T07:35:00.000Z',
      updated_at: '2026-09-03T07:35:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(findButtonByText(rendered.container, 'Sent outside Helpin')).toBeTruthy()
    expect(rendered.container.textContent).not.toContain('Sent via email')
    expect(rendered.container.textContent).not.toContain('Delivered')
    expect(rendered.container.textContent).not.toContain('Read via email')
    rendered.cleanup()
  })

  it('does not keep showing delivered email from an expired undo window', () => {
    const message: SupportMessage = {
      id: 'msg-expired-cancellable-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'viewer-1',
      sender_display_name: 'Viewer',
      content: 'Earlier email reply.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'widget',
      cancellable_until: '2026-04-24T12:20:00.000Z',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:20:00.000Z',
    }

    const expired = renderBubble(message)
    expect(expired.container.textContent).not.toContain('Delivered to email')
    expect(expired.container.textContent).not.toContain('Undo')
    expired.cleanup()

    const active = renderBubble({ ...message, id: 'msg-active-cancellable-1', cancellable_until: '2099-04-24T12:20:00.000Z' })
    expect(active.container.textContent).toContain('Queued for email')
    expect(active.container.textContent).toContain('Undo')
    expect(active.container.textContent).not.toContain('Delivered to email')
    active.cleanup()
  })

  const explicitReply: SupportMessage = {
    id: 'explicit-reply', workspace_id: 'ws-1', conversation_id: 'conv-1',
    sender_type: 'user', sender_user_id: 'another-teammate', content: 'Your update.',
    message_type: 'reply', is_internal: false, via_channel: 'email',
    metadata: JSON.stringify({ delivery_mode: 'email_only' }),
    created_at: '2026-09-03T07:35:00.000Z', updated_at: '2026-09-03T07:35:00.000Z',
  }

  it('keeps email-only intent and queued status visible to another teammate on older replies', () => {
    const rendered = renderBubble({ ...explicitReply, email_delivery_status: 'queued', cancellable_until: '2099-04-24T12:20:00.000Z' })
    expect(rendered.container.textContent).toContain('Email only · Queued')
    expect(rendered.container.textContent).not.toContain('Sent via email')
    expect(rendered.container.textContent).not.toContain('Undo')
    rendered.cleanup()
  })

  it.each([
    [{}, 'Pending'],
    [{ email_notified_at: '2026-09-03T07:36:00.000Z' }, 'Sent'],
    [{ email_delivery_status: 'delivered' }, 'Delivered'],
    [{ email_delivery_status: 'opened' }, 'Opened'],
    [{ email_delivery_status: 'failed', email_delivery_error: 'Provider unavailable' }, 'Failed'],
  ])('shows truthful email-only delivery state and ignores chat receipts (%j)', (fields, label) => {
    const rendered = renderBubble({ ...explicitReply, ...fields }, 'read', { source: 'widget' })
    expect(rendered.container.textContent).toContain(`Email only · ${label}`)
    expect(rendered.container.textContent).not.toContain('Read in chat')
    expect(rendered.container.textContent).not.toContain('Sent via email')
    rendered.cleanup()
  })

  it('shows chat delivery alongside email failure instead of hiding the successful channel', () => {
    const rendered = renderBubble({
      ...explicitReply, metadata: JSON.stringify({ delivery_mode: 'chat_and_email' }),
      email_delivery_status: 'bounced', email_delivery_error: 'Mailbox unavailable',
    }, 'read', { source: 'widget' })
    expect(rendered.container.textContent).toContain('Chat · Seen')
    expect(rendered.container.textContent).toContain('Email · Failed')
    expect(rendered.container.textContent).toContain('Mailbox unavailable')
    rendered.cleanup()
  })

  it('keeps chat-only labels on replies without a latest-message receipt', () => {
    const rendered = renderBubble({ ...explicitReply, via_channel: 'widget', metadata: JSON.stringify({ delivery_mode: 'chat_only' }) })
    expect(rendered.container.textContent).toContain('Chat only · Sent')
    expect(rendered.container.textContent).not.toContain('Email only')
    rendered.cleanup()
  })

  it('keeps chat seen status separate from email tracking on older replies', () => {
    const rendered = renderBubble({
      ...explicitReply, metadata: JSON.stringify({ delivery_mode: 'chat_and_email' }),
      email_delivery_status: 'sent',
    }, undefined, { source: 'widget', contactLastSeenAt: '2026-09-03T07:36:00.000Z' })
    expect(rendered.container.textContent).toContain('Chat · Seen')
    expect(rendered.container.textContent).toContain('Email · Sent')
    rendered.cleanup()
  })

  it.each([
    ['queued', 'Queued'], ['failed', 'Failed'], ['blocked', 'Not sent'],
  ])('shows persisted email %s outcomes to teammates after the Undo deadline', (status, label) => {
    const rendered = renderBubble({
      ...explicitReply,
      cancellable_until: '2026-04-24T12:20:00.000Z',
      metadata: JSON.stringify({ delivery_mode: 'email_only', email_delivery_status: status, email_delivery_error: status === 'blocked' ? 'Customer unsubscribed' : undefined }),
    })
    expect(rendered.container.textContent).toContain(`Email only · ${label}`)
    if (status === 'blocked') expect(rendered.container.textContent).toContain('Customer unsubscribed')
    expect(rendered.container.textContent).not.toContain('Undo')
    rendered.cleanup()
  })

  it('uses provider delivery tracking over earlier queued metadata', () => {
    const rendered = renderBubble({
      ...explicitReply, email_delivery_status: 'delivered',
      metadata: JSON.stringify({ delivery_mode: 'email_only', email_delivery_status: 'queued' }),
    })
    expect(rendered.container.textContent).toContain('Email only · Delivered')
    expect(rendered.container.textContent).not.toContain('Queued')
    rendered.cleanup()
  })

  it('keeps text and image attachments together in the bubble with message actions', () => {
    const message: SupportMessage = {
      id: 'msg-with-image-attachment',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Customer',
      content: 'Please check this screenshot.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'widget',
      attachments: [
        {
          id: 'att-image-1',
          file_key: 'support/att-image-1',
          file_name: 'screenshot.png',
          file_type: 'image/png',
          file_size: 2048,
          url: 'https://cdn.example.com/screenshot.png',
        },
      ],
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const { container, cleanup } = renderBubble(message)
    const bubbleFrame = container.querySelector('[data-slot="support-message-bubble-frame"]')
    const actions = container.querySelector('[aria-label="Message actions"]')
    const attachment = container.querySelector('img[alt="screenshot.png"]')

    expect(bubbleFrame).toBeTruthy()
    expect(bubbleFrame?.className).toContain('relative')
    expect(actions).toBeTruthy()
    expect(attachment).toBeTruthy()
    expect(bubbleFrame?.contains(actions)).toBe(true)
    expect(bubbleFrame?.contains(attachment)).toBe(true)

    cleanup()
  })

  it('defers support image downloads until the thumbnail is near the viewport', () => {
    let intersect: IntersectionObserverCallback | undefined
    const originalIntersectionObserver = globalThis.IntersectionObserver
    globalThis.IntersectionObserver = class IntersectionObserver {
      readonly root = null
      readonly rootMargin = '240px'
      readonly thresholds = [0]
      constructor(callback: IntersectionObserverCallback) {
        intersect = callback
      }
      disconnect() {}
      observe() {}
      takeRecords() { return [] }
      unobserve() {}
    }

    const message: SupportMessage = {
      id: 'msg-deferred-image',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      content: '',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'widget',
      attachments: [{
        id: 'att-deferred-image',
        file_key: 'support/att-deferred-image',
        file_name: 'large-photo.png',
        file_type: 'image/png',
        file_size: 8_000_000,
        url: 'https://cdn.example.com/large-photo.png',
      }],
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const rendered = renderBubble(message)
    const image = rendered.container.querySelector('img[alt="large-photo.png"]') as HTMLImageElement
    expect(image.getAttribute('src')).toBeNull()
    expect(image.getAttribute('decoding')).toBe('async')
    expect(image.getAttribute('fetchpriority')).toBe('low')

    act(() => {
      intersect?.([{ isIntersecting: true, target: image } as IntersectionObserverEntry], {} as IntersectionObserver)
    })
    expect(image.getAttribute('src')).toBe('https://cdn.example.com/large-photo.png')

    rendered.cleanup()
    globalThis.IntersectionObserver = originalIntersectionObserver
  })

  it('renders internal note images as thumbnails with hover preview and image navigation', () => {
    const message: SupportMessage = {
      id: 'msg-note-attachments-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'viewer-1',
      sender_display_name: 'Viewer',
      content: '',
      message_type: 'note',
      is_internal: true,
      via_channel: 'widget',
      attachments: [
        {
          id: 'att-image-1',
          file_key: 'support/att-image-1',
          file_name: 'screenshot.png',
          file_type: 'image/png',
          file_size: 2048,
          url: 'https://cdn.example.com/screenshot.png',
        },
        {
          id: 'att-image-2',
          file_key: 'support/att-image-2',
          file_name: 'receipt.png',
          file_type: 'image/png',
          file_size: 3072,
          url: 'https://cdn.example.com/receipt.png',
        },
        {
          id: 'att-file-1',
          file_key: 'support/att-file-1',
          file_name: 'diagnostics.pdf',
          file_type: 'application/pdf',
          file_size: 4096,
          url: 'https://cdn.example.com/diagnostics.pdf',
        },
      ],
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const { container, cleanup } = renderBubble(message)

    const noteCard = container.querySelector('.border-r-amber-400')
    expect(noteCard).toBeTruthy()

    const fileLink = container.querySelector('a[href="https://cdn.example.com/diagnostics.pdf"]')
    expect(fileLink?.textContent).toContain('diagnostics.pdf')
    expect(fileLink?.className).toContain('border-amber-200')

    const image = container.querySelector('img[alt="screenshot.png"]') as HTMLImageElement | null
    expect(image).toBeTruthy()
    expect(image?.getAttribute('src')).toBe('https://cdn.example.com/screenshot.png')
    expect(image?.className).toContain('h-full')
    expect(image?.className).toContain('w-full')
    expect(image?.closest('button')?.className).toContain('h-16')

    act(() => {
      image?.closest('button')?.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    })

    const hoverPreview = container.querySelector('[data-testid="support-attachment-hover-preview"] img') as HTMLImageElement | null
    expect(hoverPreview?.getAttribute('src')).toBe('https://cdn.example.com/screenshot.png')
    expect(container.textContent).toContain('1 / 2')

    const hoverPreviewBridge = container.querySelector('[data-testid="support-attachment-hover-preview"]')
    expect(hoverPreviewBridge?.className).toContain('pb-2')
    expect(hoverPreviewBridge?.className).not.toContain('mb-2')

    const hoverNext = container.querySelector('button[aria-label="Next image attachment"]')
    act(() => {
      hoverNext?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    const nextHoverPreview = container.querySelector('[data-testid="support-attachment-hover-preview"] img') as HTMLImageElement | null
    expect(nextHoverPreview?.getAttribute('src')).toBe('https://cdn.example.com/receipt.png')

    act(() => {
      image?.closest('button')?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const preview = document.body.querySelector('[data-testid="support-attachment-lightbox"] img') as HTMLImageElement | null
    expect(preview).toBeTruthy()
    expect(preview?.getAttribute('src')).toBe('https://cdn.example.com/screenshot.png')
    expect(document.body.textContent).toContain('screenshot.png')

    const lightboxNext = document.body.querySelector('[data-testid="support-attachment-lightbox"] button[aria-label="Next image attachment"]')
    act(() => {
      lightboxNext?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    const nextPreview = document.body.querySelector('[data-testid="support-attachment-lightbox"] img') as HTMLImageElement | null
    expect(nextPreview?.getAttribute('src')).toBe('https://cdn.example.com/receipt.png')

    cleanup()
  })

  it('constrains markdown images in internal notes', () => {
    const message: SupportMessage = {
      id: 'msg-note-markdown-image-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'viewer-1',
      sender_display_name: 'Viewer',
      content: 'Here is the screenshot:\n\n![Inline screenshot](https://cdn.example.com/inline.png)',
      message_type: 'note',
      is_internal: true,
      via_channel: 'widget',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const { container, cleanup } = renderBubble(message)

    const image = container.querySelector('img[alt="Inline screenshot"]') as HTMLImageElement | null
    expect(image).toBeTruthy()
    expect(image?.getAttribute('loading')).toBe('lazy')
    expect(image?.className).toContain('max-h-60')
    expect(image?.className).toContain('max-w-full')

    cleanup()
  })

  it('renders outbound fallback email failure states ahead of read receipts', () => {
    const message: SupportMessage = {
      id: 'msg-email-status-3',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Agent',
      content: 'Following up here.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'widget',
      email_notified_at: '2026-04-24T12:20:00.000Z',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:20:00.000Z',
    }

    const bounced = renderBubble({
      ...message,
      email_delivery_status: 'bounced',
      email_delivery_error: 'Mailbox unavailable',
    }, 'read_email')
    expect(bounced.container.textContent).toContain('Delivery failed · Mailbox unavailable')
    expect(bounced.container.textContent).not.toContain('Read via email')
    bounced.cleanup()

    const spam = renderBubble({
      ...message,
      id: 'msg-email-status-4',
      email_delivery_status: 'spam_complaint',
      email_delivery_error: 'Marked by recipient',
    }, 'delivered_email')
    expect(spam.container.textContent).toContain('Marked as spam · Marked by recipient')
    expect(spam.container.textContent).not.toContain('Delivered via email')
    spam.cleanup()
  })
})

it.each([false, true])('attributes participant mail and preserves team-only privacy (%s)', (unknown) => {
  const message: SupportMessage = {
    id: 'participant-email', workspace_id: 'ws-1', conversation_id: 'conv-1',
    sender_type: 'customer', sender_display_name: 'Colleague', content: 'My reply',
    message_type: 'reply', is_internal: unknown, via_channel: 'email',
    email_from: 'colleague@example.com', email_to: 'support@example.com', email_cc: ['customer@example.com'],
    metadata: JSON.stringify({ email_sender: 'colleague@example.com', email_participant_sender: true, email_unknown_sender: unknown }),
    created_at: '2026-09-21T09:00:00Z', updated_at: '2026-09-21T09:00:00Z',
  }
  const rendered = renderBubble(message, undefined, { customerEmail: 'customer@example.com', fallbackAvatarUrl: 'https://example.com/customer-avatar.png' })
  try {
    expect(rendered.container.querySelector('img[src="https://example.com/customer-avatar.png"]')).toBeNull()
    if (unknown) expect(rendered.container.textContent).toContain('Team only')
    expect(rendered.container.querySelector('details')).toBeNull()
    expect(rendered.container.textContent).not.toContain('To: support@example.com')
    if (!unknown) expect(rendered.container.textContent).not.toContain('Colleague')
  } finally { rendered.cleanup() }
})
