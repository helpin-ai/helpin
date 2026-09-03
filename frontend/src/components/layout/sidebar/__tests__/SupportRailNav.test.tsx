// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { SidebarProvider } from '@/components/ui/sidebar'
import { TooltipProvider } from '@/components/ui/tooltip'
import { SupportRailNav } from '../SupportRailNav'
import type { SupportInboxView, SupportInboxViewCount } from '@/lib/pmTypes'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})

function renderSupportRail({
  customViews = [],
  customViewCounts = {},
  canManageSettings = false,
}: {
  customViews?: SupportInboxView[]
  customViewCounts?: Record<string, SupportInboxViewCount>
  canManageSettings?: boolean
} = {}) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(
      <TooltipProvider>
        <SidebarProvider>
          <SupportRailNav
            navFilter="inbox"
            selectedMailboxId="mailbox-test"
            activeCustomViewId={null}
            unreadStats={{
              inbox: 1,
              mine: 4,
              waiting: 0,
              ai_active: 0,
              inbox_total: 2,
              mine_total: 8,
              waiting_total: 0,
              ai_active_total: 0,
              total: 1,
              my_inbox: 0,
              unassigned: 0,
            }}
            inboxUnreadStats={{
              inbox: 2,
              mine: 0,
              waiting: 0,
              ai_active: 0,
              inbox_total: 9,
              mine_total: 0,
              waiting_total: 0,
              ai_active_total: 0,
              total: 2,
              my_inbox: 0,
              unassigned: 0,
              inbox_needs_human_reply: 0,
            }}
            globalUnreadStats={{
              inbox: 3,
              mine: 1,
              waiting: 0,
              ai_active: 1,
              inbox_total: 5,
              mine_total: 2,
              waiting_total: 0,
              ai_active_total: 4,
              total: 3,
              my_inbox: 1,
              unassigned: 1,
              mine_needs_human_reply: 0,
            }}
            inboxScopes={{
              mailboxes: [
                {
                  id: 'mailbox-test',
                  name: 'Test',
                  total_count: 2,
                  unread_count: 1,
                  needs_human_reply_count: 0,
                },
              ],
            }}
            customViews={customViews}
            customViewCounts={customViewCounts}
            canManageSettings={canManageSettings}
            wsSlug="test-docs"
            pathname="/w/test-docs/support"
            onNavFilterChange={vi.fn()}
            onMailboxSelect={vi.fn()}
            onCustomViewSelect={vi.fn()}
            onEditCustomView={vi.fn()}
            onDeleteCustomView={vi.fn()}
            onCreateMailbox={vi.fn()}
            onEditMailbox={vi.fn()}
            onArchiveMailbox={vi.fn()}
            onNavigate={vi.fn()}
          />
        </SidebarProvider>
      </TooltipProvider>,
    )
  })

  return {
    container,
    cleanup: () => {
      act(() => root.unmount())
      container.remove()
    },
  }
}

function buttonByText(container: HTMLElement, text: string) {
  return Array.from(container.querySelectorAll('button')).find((button) =>
    button.textContent?.includes(text),
  ) as HTMLButtonElement | undefined
}

describe('SupportRailNav', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('keeps team inbox active without scoping the AI handling badge', () => {
    const rendered = renderSupportRail()

    expect(buttonByText(rendered.container, 'Inbox')?.dataset.active).toBe('false')
    expect(buttonByText(rendered.container, 'Test')?.dataset.active).toBe('true')

    const aiHandling = buttonByText(rendered.container, 'AI Handling')
    expect(aiHandling?.textContent).toContain('4')
    expect(aiHandling?.textContent).not.toContain('1')
    expect(aiHandling?.querySelector('[data-slot="support-unread-dot"]')).toBeNull()
    expect(aiHandling?.querySelector('[data-slot="support-total-count"]')?.textContent).toBe('4')

    rendered.cleanup()
  })

  it('uses the separately scoped Inbox count and global Mine count', () => {
    const rendered = renderSupportRail()

    expect(buttonByText(rendered.container, 'Inbox')?.textContent).toContain('9')
    expect(buttonByText(rendered.container, 'Mine')?.textContent).toContain('2')
    expect(buttonByText(rendered.container, 'Mine')?.textContent).not.toContain('8')

    rendered.cleanup()
  })

  it('shows red dots from personal unread counts rather than team reply state', () => {
    const rendered = renderSupportRail()

    expect(buttonByText(rendered.container, 'Inbox')?.querySelector('[data-slot="support-unread-dot"]')).toBeTruthy()
    expect(buttonByText(rendered.container, 'Mine')?.querySelector('[data-slot="support-unread-dot"]')).toBeTruthy()
    expect(buttonByText(rendered.container, 'Test')?.querySelector('[data-slot="support-unread-dot"]')).toBeTruthy()

    rendered.cleanup()
  })

  it('does not expose support global search as a sidebar list item', () => {
    const rendered = renderSupportRail()

    expect(buttonByText(rendered.container, 'Search')).toBeUndefined()

    rendered.cleanup()
  })

  it('shows custom views below team inboxes', () => {
    const rendered = renderSupportRail({
      customViews: [
        {
          id: 'view-1',
          workspace_id: 'ws-1',
          name: 'Escalations',
          filters: {},
          is_shared: false,
          view_type: 'custom',
          created_by: 'user-1',
          created_at: '2026-05-04T00:00:00Z',
          updated_at: '2026-05-04T00:00:00Z',
        },
      ],
    })

    const text = rendered.container.textContent ?? ''
    expect(text.indexOf('Team Inboxes')).toBeGreaterThan(-1)
    expect(text.indexOf('Custom views')).toBeGreaterThan(-1)
    expect(text.indexOf('Custom views')).toBeGreaterThan(text.indexOf('Team Inboxes'))

    rendered.cleanup()
  })

  it('shows custom view workload count and unread dot', () => {
    const rendered = renderSupportRail({
      customViews: [
        {
          id: 'view-1',
          workspace_id: 'ws-1',
          name: 'Escalations',
          filters: {},
          is_shared: false,
          view_type: 'custom',
          created_by: 'user-1',
          created_at: '2026-05-04T00:00:00Z',
          updated_at: '2026-05-04T00:00:00Z',
        },
      ],
      customViewCounts: {
        'view-1': {
          view_id: 'view-1',
          total_count: 7,
          unread_count: 2,
        },
      },
    })

    const customView = buttonByText(rendered.container, 'Escalations')
    expect(customView?.textContent).toContain('7')
    expect(customView?.querySelector('[data-slot="support-unread-dot"]')).toBeTruthy()

    rendered.cleanup()
  })

  it('uses a shared trailing slot for team inbox counts', () => {
    const rendered = renderSupportRail({ canManageSettings: true })

    const teamInbox = buttonByText(rendered.container, 'Test')
    const teamCountSlot = teamInbox?.querySelector('[data-slot="support-total-count"]')?.parentElement
    const inboxCountSlot = buttonByText(rendered.container, 'Inbox')
      ?.querySelector('[data-slot="support-total-count"]')
      ?.parentElement

    expect(inboxCountSlot?.className).toContain('min-w-4')
    expect(inboxCountSlot?.className).toContain('justify-center')
    expect(teamCountSlot?.className).toContain('min-w-4')
    expect(teamCountSlot?.className).toContain('justify-center')

    rendered.cleanup()
  })

  it('matches the Docs and PM sidebar plus button styling', () => {
    const rendered = renderSupportRail({ canManageSettings: true })

    const createAction = rendered.container.querySelector('[data-slot="support-create-team-inbox-action"]')
    const createIcon = createAction?.querySelector('svg')

    expect(createAction?.className).toContain('group/plus')
    expect(createAction?.className).toContain('-mr-0.5')
    expect(createAction?.className).toContain('h-5')
    expect(createAction?.className).toContain('w-5')
    expect(createAction?.className).toContain('hover:bg-muted')
    expect(createIcon?.className.baseVal).toContain('h-3')
    expect(createIcon?.className.baseVal).toContain('w-3')
    expect(createIcon?.className.baseVal).toContain('group-hover/plus:text-foreground')

    rendered.cleanup()
  })

  it('keeps the action footer in sidebar flow instead of overlaying the trial banner', () => {
    const rendered = renderSupportRail()
    const footer = rendered.container.querySelector('.support-bottom-bar')

    expect(footer?.classList.contains('sticky')).toBe(true)
    expect(footer?.classList.contains('fixed')).toBe(false)
    expect(footer?.classList.contains('mt-auto')).toBe(true)

    rendered.cleanup()
  })
})
