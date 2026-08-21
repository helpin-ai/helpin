import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import {
  useCreateSupportCannedResponse,
  useDeleteSupportCannedResponse,
  useUpdateSupportCannedResponse,
  type SupportCannedResponse,
} from '@helpin-ai/support-core'

import { CannedResponsesSheet, validateShortcutCode } from '../canned-responses-sheet'

vi.mock('@helpin-ai/support-core', () => ({
  useCreateSupportCannedResponse: vi.fn(),
  useUpdateSupportCannedResponse: vi.fn(),
  useDeleteSupportCannedResponse: vi.fn(),
}))

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const RESPONSE: SupportCannedResponse = {
  id: 'shortcut-1',
  workspace_id: 'ws-1',
  short_code: '!hello',
  content: 'Hello {{customer.first_name}}',
  tag: 'Support',
  created_by_id: 'user-1',
  created_at: '2026-08-20T00:00:00.000Z',
  updated_at: '2026-08-20T00:00:00.000Z',
}

const create = vi.fn(async () => RESPONSE)
const update = vi.fn(async () => RESPONSE)
const remove = vi.fn(async () => ({ message: 'deleted' }))

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useCreateSupportCannedResponse).mockReturnValue({ mutateAsync: create, isPending: false } as unknown as ReturnType<typeof useCreateSupportCannedResponse>)
  vi.mocked(useUpdateSupportCannedResponse).mockReturnValue({ mutateAsync: update, isPending: false } as unknown as ReturnType<typeof useUpdateSupportCannedResponse>)
  vi.mocked(useDeleteSupportCannedResponse).mockReturnValue({ mutateAsync: remove, isPending: false } as unknown as ReturnType<typeof useDeleteSupportCannedResponse>)
})

test('validates shortcut codes with the same rules as web', () => {
  expect(validateShortcutCode('')).toBe('Shortcut is required')
  expect(validateShortcutCode('hello')).toBe('Shortcut must start with !')
  expect(validateShortcutCode('!hello there')).toBe('Shortcut cannot contain spaces')
  expect(validateShortcutCode('!')).toBe('Shortcut must have at least one character after !')
  expect(validateShortcutCode('!hello')).toBeNull()
})

test('Support admins can create categorized shortcuts with compatibility title', async () => {
  render(
    <CannedResponsesSheet
      workspaceId="ws-1"
      open
      onOpenChange={() => {}}
      responses={[]}
      canManage
      onSelect={() => {}}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Create shortcut' }))
  fireEvent.change(screen.getByRole('textbox', { name: 'Shortcut' }), { target: { value: '!refund' } })
  fireEvent.change(screen.getByRole('combobox', { name: 'Category' }), { target: { value: 'Billing' } })
  fireEvent.change(screen.getByRole('textbox', { name: 'Shortcut message' }), { target: { value: 'Your refund is ready.' } })
  fireEvent.click(screen.getByRole('button', { name: 'Create' }))

  await waitFor(() => expect(create).toHaveBeenCalledWith({
    short_code: '!refund',
    content: 'Your refund is ready.',
    tag: 'Billing',
    title: '!refund',
  }))
})

test('Support admins can edit and two-step delete an existing shortcut', async () => {
  render(
    <CannedResponsesSheet
      workspaceId="ws-1"
      open
      onOpenChange={() => {}}
      responses={[RESPONSE]}
      canManage
      onSelect={() => {}}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Edit !hello' }))
  fireEvent.change(screen.getByRole('textbox', { name: 'Shortcut message' }), { target: { value: 'Hello there' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(update).toHaveBeenCalledWith({
    responseId: 'shortcut-1',
    payload: { short_code: '!hello', content: 'Hello there', tag: 'Support', title: '!hello' },
  }))

  fireEvent.click(screen.getByRole('button', { name: 'Edit !hello' }))
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
  expect(remove).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Confirm delete' }))
  await waitFor(() => expect(remove).toHaveBeenCalledWith('shortcut-1'))
})

test('non-admins can insert shortcuts but do not see management controls', () => {
  const select = vi.fn()
  render(
    <CannedResponsesSheet
      workspaceId="ws-1"
      open
      onOpenChange={() => {}}
      responses={[RESPONSE]}
      onSelect={select}
    />,
  )

  expect(screen.queryByRole('button', { name: 'Create shortcut' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Edit !hello' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: '!hello' }))
  expect(select).toHaveBeenCalledWith(RESPONSE)
})
