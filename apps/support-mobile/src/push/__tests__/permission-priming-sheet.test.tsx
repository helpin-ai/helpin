import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { toast } from 'sonner'
import { setPushPrimingPref } from '@mobile/lib/prefs'
import { registerForPush } from '@mobile/push/push-registration'
import { PermissionPrimingSheet } from '../permission-priming-sheet'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

vi.mock('@mobile/lib/prefs', () => ({
  setPushPrimingPref: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('@mobile/push/push-registration', () => ({
  registerForPush: vi.fn(),
}))

const mockToastError = vi.mocked(toast.error)
const mockSetPushPrimingPref = vi.mocked(setPushPrimingPref)
const mockRegisterForPush = vi.mocked(registerForPush)

beforeEach(() => {
  mockToastError.mockClear()
  mockSetPushPrimingPref.mockClear()
  mockRegisterForPush.mockReset()
})

/**
 * Task 22 fold-in: `registerForPush()` can REJECT (e.g. the native plugin's
 * `getPushToken()` throwing) rather than resolving to 'unavailable'. Before
 * this fix, `handleEnable`'s only `try`/`finally` let that rejection escape
 * as an unhandled promise rejection (this handler is fired via
 * `void handleEnable()`) — the sheet still closed, but the user never saw
 * any feedback and no pref was persisted either way. This test drives that
 * exact path and asserts the rejection is caught, surfaced as a toast, and
 * never reaches an unhandled-rejection state (which vitest/jsdom would
 * otherwise flag).
 */
test('a rejected registerForPush() is caught and surfaced as an error toast, not left unhandled', async () => {
  mockRegisterForPush.mockRejectedValue(new Error('getPushToken threw'))
  const onOpenChange = vi.fn()

  render(<PermissionPrimingSheet open onOpenChange={onOpenChange} />)
  fireEvent.click(screen.getByRole('button', { name: 'Enable notifications' }))

  await waitFor(() => expect(mockToastError).toHaveBeenCalledWith("Couldn't enable notifications"))
  expect(onOpenChange).toHaveBeenCalledWith(false)
  // A rejected attempt is not a confirmed registration — never persist 'enabled'.
  expect(mockSetPushPrimingPref).not.toHaveBeenCalled()
})

test('a resolved "registered" result persists the enabled decision and shows no toast', async () => {
  mockRegisterForPush.mockResolvedValue('registered')
  const onOpenChange = vi.fn()

  render(<PermissionPrimingSheet open onOpenChange={onOpenChange} />)
  fireEvent.click(screen.getByRole('button', { name: 'Enable notifications' }))

  await waitFor(() =>
    expect(mockSetPushPrimingPref).toHaveBeenCalledWith(expect.objectContaining({ decision: 'enabled' })),
  )
  expect(mockToastError).not.toHaveBeenCalled()
})
