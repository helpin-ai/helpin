import { render, screen, fireEvent } from '@testing-library/react'
import {
  useInboxScopes,
  useSupportBuiltinInboxViews,
  useSupportInboxViewCounts,
  useSupportInboxViews,
} from '@helpin-ai/support-core'
import { ViewsDrawer } from '../views-drawer'
import type { ViewSelection } from '../use-inbox-filters'

vi.mock('@helpin-ai/support-core', () => ({
  useSupportBuiltinInboxViews: vi.fn(),
  useSupportInboxViewCounts: vi.fn(),
  useSupportInboxViews: vi.fn(),
  useInboxScopes: vi.fn(),
}))
vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))

const mockBuiltins = vi.mocked(useSupportBuiltinInboxViews)
const mockCounts = vi.mocked(useSupportInboxViewCounts)
const mockCustom = vi.mocked(useSupportInboxViews)
const mockScopes = vi.mocked(useInboxScopes)

function builtin(navKey: string, id: string) {
  return { id, workspace_id: 'ws', name: navKey, filters: {}, is_shared: false, view_type: 'default', view_key: `nav:${navKey}`, created_by: 'u', created_at: '', updated_at: '' }
}

beforeEach(() => {
  mockBuiltins.mockReturnValue({ data: [builtin('inbox', 'v-inbox'), builtin('waiting', 'v-waiting')] } as never)
  mockCounts.mockReturnValue({ data: [{ view_id: 'v-inbox', total_count: 9, unread_count: 4 }] } as never)
  mockCustom.mockReturnValue({ data: [] } as never)
  mockScopes.mockReturnValue({ data: undefined, isLoading: false } as never)
})

const active: ViewSelection = { kind: 'builtin', navFilter: 'inbox', mailboxId: 'all' }

test('renders the Views and AI groups with the total count number and an unread dot', () => {
  render(<ViewsDrawer open onOpenChange={vi.fn()} workspaceId="ws" activeSelection={active} onSelect={vi.fn()} />)
  expect(screen.getByText('Views')).toBeTruthy()
  expect(screen.getByText('AI')).toBeTruthy()
  expect(screen.getByText('Inbox')).toBeTruthy()
  expect(screen.getByText('AI Handling')).toBeTruthy()
  // Inbox: the number badge is the TOTAL (9), with a red dot for the 4 unread (mirrors web).
  expect(screen.getByText('9')).toBeTruthy()
  expect(screen.getByLabelText('4 unread')).toBeTruthy()
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
