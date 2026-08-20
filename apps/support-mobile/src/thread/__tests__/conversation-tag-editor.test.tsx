import { fireEvent, render, screen } from '@testing-library/react'
import {
  useAddConversationTag,
  useRemoveConversationTag,
  useSupportTags,
} from '@helpin-ai/support-core'
import { ConversationTagEditor } from '../conversation-tag-editor'

vi.mock('@helpin-ai/support-core', () => ({
  useSupportTags: vi.fn(),
  useAddConversationTag: vi.fn(),
  useRemoveConversationTag: vi.fn(),
}))

vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))

const mockUseSupportTags = vi.mocked(useSupportTags)
const mockUseAddConversationTag = vi.mocked(useAddConversationTag)
const mockUseRemoveConversationTag = vi.mocked(useRemoveConversationTag)

const TAGS = [
  { id: 'tag-billing', name: 'Billing', color: '#2563eb' },
  { id: 'tag-vip', name: 'VIP', color: '#dc2626' },
]

function setup() {
  const add = vi.fn()
  const remove = vi.fn()
  mockUseSupportTags.mockReturnValue({
    data: TAGS, isPending: false, isError: false, refetch: vi.fn(),
  } as unknown as ReturnType<typeof useSupportTags>)
  mockUseAddConversationTag.mockReturnValue({ mutate: add, isPending: false } as unknown as ReturnType<typeof useAddConversationTag>)
  mockUseRemoveConversationTag.mockReturnValue({ mutate: remove, isPending: false } as unknown as ReturnType<typeof useRemoveConversationTag>)
  return { add, remove }
}

beforeEach(() => vi.clearAllMocks())

test('shows selected tags and removes one through the conversation endpoint hook', () => {
  const { remove } = setup()
  render(<ConversationTagEditor workspaceId="ws-1" conversationId="conv-1" selectedTags={[TAGS[0]]} />)

  expect(screen.getAllByText('Billing')).toHaveLength(2)
  fireEvent.click(screen.getByRole('button', { name: 'Remove tag Billing' }))

  expect(remove).toHaveBeenCalledWith(
    { conversationId: 'conv-1', tagId: 'tag-billing' },
    expect.anything(),
  )
})

test('adds an unselected existing tag', () => {
  const { add } = setup()
  render(<ConversationTagEditor workspaceId="ws-1" conversationId="conv-1" selectedTags={[TAGS[0]]} />)

  fireEvent.click(screen.getByRole('button', { name: 'Add tag VIP' }))

  expect(add).toHaveBeenCalledWith(
    { conversationId: 'conv-1', tagId: 'tag-vip' },
    expect.anything(),
  )
})

test('filters the existing tag list by name', () => {
  setup()
  render(<ConversationTagEditor workspaceId="ws-1" conversationId="conv-1" selectedTags={[]} />)

  fireEvent.change(screen.getByPlaceholderText('Search tags'), { target: { value: 'vip' } })

  expect(screen.getByRole('button', { name: 'Add tag VIP' })).toBeDefined()
  expect(screen.queryByRole('button', { name: 'Add tag Billing' })).toBeNull()
})
