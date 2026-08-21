import { render, screen, fireEvent } from '@testing-library/react'
import {
  useInboxScopes,
  useSupportInboxViewCounts,
  useSupportInboxViews,
  useUnreadStats,
} from '@helpin-ai/support-core'
import { ViewsDrawer } from '../views-drawer'
import type { ViewSelection } from '../use-inbox-filters'
import type { Workspace } from '@mobile/lib/types'

vi.mock('@helpin-ai/support-core', () => ({
  useUnreadStats: vi.fn(),
  useSupportInboxViewCounts: vi.fn(),
  useSupportInboxViews: vi.fn(),
  useInboxScopes: vi.fn(),
}))
vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))

const mockUnreadStats = vi.mocked(useUnreadStats)
const mockCounts = vi.mocked(useSupportInboxViewCounts)
const mockCustom = vi.mocked(useSupportInboxViews)
const mockScopes = vi.mocked(useInboxScopes)

beforeEach(() => {
  // Inbox builtin count now comes from unread-stats: number = inbox_total (9), dot = inbox unread (4).
  mockUnreadStats.mockReturnValue({
    data: { total: 0, my_inbox: 0, unassigned: 0, inbox: 4, mine: 0, waiting: 0, ai_active: 0, inbox_total: 9, mine_total: 0, waiting_total: 0, ai_active_total: 0 },
  } as never)
  mockCounts.mockReturnValue({ data: [] } as never)
  mockCustom.mockReturnValue({ data: [] } as never)
  mockScopes.mockReturnValue({ data: undefined, isLoading: false } as never)
})

const active: ViewSelection = { kind: 'builtin', navFilter: 'inbox', mailboxId: 'all' }
const workspaces: Workspace[] = [
  { id: 'ws', name: 'Test Docs', slug: 'test-docs' },
  { id: 'other', name: 'Customer Success', slug: 'customer-success' },
]

test('renders built-in views as one unlabeled list with the total count number and an unread dot', () => {
  render(<ViewsDrawer open onOpenChange={vi.fn()} workspaceId="ws" activeSelection={active} onSelect={vi.fn()} />)
  expect(screen.queryByText('Views')).toBeNull()
  expect(screen.queryByText('AI')).toBeNull()
  expect(screen.getByText('Inbox')).toBeTruthy()
  expect(screen.getByText('AI Handling')).toBeTruthy()
  // Inbox: the number badge is the TOTAL (9), with a red dot for the 4 unread (mirrors web).
  expect(screen.getByText('9')).toBeTruthy()
  expect(screen.getByLabelText('4 unread')).toBeTruthy()
  const inboxRow = screen.getByRole('button', { name: /Inbox/ })
  expect(inboxRow.className).toContain('w-[calc(100%-1rem)]')
  const label = screen.getByText('Inbox')
  const dot = screen.getByLabelText('4 unread')
  const count = screen.getByText('9')
  expect(label.compareDocumentPosition(dot) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(dot.compareDocumentPosition(count) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(count.className).toContain('text-footnote')
  expect(count.className).not.toContain('rounded-full')
  expect(inboxRow.lastElementChild?.contains(dot)).toBe(true)
  expect(inboxRow.lastElementChild?.contains(count)).toBe(true)
})

test('pins Settings outside the scrollable view list and closes before navigating', () => {
  const onOpenSettings = vi.fn()
  const onOpenChange = vi.fn()
  render(
    <ViewsDrawer
      open
      onOpenChange={onOpenChange}
      workspaceId="ws"
      activeSelection={active}
      onSelect={vi.fn()}
      onOpenSettings={onOpenSettings}
    />,
  )

  const settings = screen.getByRole('button', { name: 'Settings' })
  expect(settings.closest('[data-testid="drawer-footer"]')).toBeTruthy()
  fireEvent.click(settings)
  expect(onOpenChange).toHaveBeenCalledWith(false)
  expect(onOpenSettings).toHaveBeenCalledOnce()
})

test('selecting a row fires onSelect with the selection and closes the drawer', () => {
  const onSelect = vi.fn()
  const onOpenChange = vi.fn()
  render(<ViewsDrawer open onOpenChange={onOpenChange} workspaceId="ws" activeSelection={active} onSelect={onSelect} />)
  fireEvent.click(screen.getByText('Waiting'))
  expect(onSelect).toHaveBeenCalledWith({ kind: 'builtin', navFilter: 'waiting', mailboxId: 'all' })
  expect(onOpenChange).toHaveBeenCalledWith(false)
})

test('renders nothing when closed', () => {
  const { container } = render(
    <ViewsDrawer open={false} onOpenChange={vi.fn()} workspaceId="ws" activeSelection={active} onSelect={vi.fn()} />,
  )
  expect(container.querySelector('[aria-label="Inbox views"]')).toBeNull()
})

test('expands the workspace switcher from the current workspace name', () => {
  render(
    <ViewsDrawer
      open
      onOpenChange={vi.fn()}
      workspaceId="ws"
      workspace={workspaces[0]}
      workspaces={workspaces}
      activeSelection={active}
      onSelect={vi.fn()}
      onSelectWorkspace={vi.fn()}
    />,
  )

  expect(screen.queryByText('Customer Success')).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Switch workspace, currently Test Docs' }))
  expect(screen.getByText('Customer Success')).toBeTruthy()
  expect(screen.getByLabelText('Test Docs, current workspace')).toBeTruthy()
})

test('selecting a workspace switches and closes the drawer', () => {
  const onSelectWorkspace = vi.fn()
  const onOpenChange = vi.fn()
  render(
    <ViewsDrawer
      open
      onOpenChange={onOpenChange}
      workspaceId="ws"
      workspace={workspaces[0]}
      workspaces={workspaces}
      activeSelection={active}
      onSelect={vi.fn()}
      onSelectWorkspace={onSelectWorkspace}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Switch workspace, currently Test Docs' }))
  fireEvent.click(screen.getByRole('button', { name: 'Switch to Customer Success' }))

  expect(onSelectWorkspace).toHaveBeenCalledWith(workspaces[1])
  expect(onOpenChange).toHaveBeenCalledWith(false)
})

test('keeps long workspace lists scrollable within the drawer viewport', () => {
  render(
    <ViewsDrawer
      open
      onOpenChange={vi.fn()}
      workspaceId="ws"
      workspace={workspaces[0]}
      workspaces={workspaces}
      activeSelection={active}
      onSelect={vi.fn()}
      onSelectWorkspace={vi.fn()}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Switch workspace, currently Test Docs' }))
  expect(screen.getByTestId('workspace-options').className).toContain('overflow-y-auto')
  expect(screen.getByTestId('workspace-options').className).toContain('max-h-')
})

test('shows the current workspace and a retry action when loading workspaces fails', () => {
  const onRetryWorkspaces = vi.fn()
  render(
    <ViewsDrawer
      open
      onOpenChange={vi.fn()}
      workspaceId="ws"
      workspace={workspaces[0]}
      workspacesError
      onRetryWorkspaces={onRetryWorkspaces}
      activeSelection={active}
      onSelect={vi.fn()}
      onSelectWorkspace={vi.fn()}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Switch workspace, currently Test Docs' }))
  expect(screen.getByLabelText('Test Docs, current workspace')).toBeTruthy()
  expect(screen.getByText("Couldn't load workspaces")).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'Retry loading workspaces' }))
  expect(onRetryWorkspaces).toHaveBeenCalledOnce()
})
