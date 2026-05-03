// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { SidebarProvider } from '@/components/ui/sidebar'
import { TooltipProvider } from '@/components/ui/tooltip'
import { SupportRailNav } from '../SupportRailNav'

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

function renderSupportRail() {
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
              mine: 0,
              waiting: 0,
              ai_active: 0,
              inbox_total: 2,
              mine_total: 0,
              waiting_total: 0,
              ai_active_total: 0,
              total: 1,
              my_inbox: 0,
              unassigned: 0,
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
            }}
            inboxScopes={{
              mailboxes: [
                {
                  id: 'mailbox-test',
                  name: 'Test',
                  total_count: 2,
                  unread_count: 1,
                },
              ],
            }}
            customViews={[]}
            canManageSettings={false}
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
    expect(aiHandling?.querySelector('[data-slot="support-unread-dot"]')).toBeTruthy()
    expect(aiHandling?.querySelector('[data-slot="support-total-count"]')?.textContent).toBe('4')

    rendered.cleanup()
  })
})
