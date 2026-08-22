import {
  impactFeedback,
  notificationFeedback,
  selectionFeedback,
} from '@tauri-apps/plugin-haptics'
import { haptic } from '@mobile/lib/haptics'

// Mocked locally (not in a shared setup file) so this fake plugin module never
// leaks into other test files that import unrelated code.
vi.mock('@tauri-apps/plugin-haptics', () => ({
  impactFeedback: vi.fn(),
  notificationFeedback: vi.fn(),
  selectionFeedback: vi.fn(),
}))

const mockImpactFeedback = vi.mocked(impactFeedback)
const mockNotificationFeedback = vi.mocked(notificationFeedback)
const mockSelectionFeedback = vi.mocked(selectionFeedback)

function markAsTauri() {
  ;(window as unknown as { __TAURI_INTERNALS__: unknown }).__TAURI_INTERNALS__ = {}
}

function clearTauriFlag() {
  delete (window as unknown as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__
}

beforeEach(() => {
  clearTauriFlag()
  mockImpactFeedback.mockReset().mockResolvedValue({ status: 'ok', data: null })
  mockNotificationFeedback.mockReset().mockResolvedValue({ status: 'ok', data: null })
  mockSelectionFeedback.mockReset().mockResolvedValue({ status: 'ok', data: null })
})

afterEach(() => {
  clearTauriFlag()
})

test('no-ops outside Tauri (no __TAURI_INTERNALS__ on window)', () => {
  expect(() => haptic('selection')).not.toThrow()
  expect(mockSelectionFeedback).not.toHaveBeenCalled()
  expect(mockImpactFeedback).not.toHaveBeenCalled()
  expect(mockNotificationFeedback).not.toHaveBeenCalled()
})

test("haptic('selection') calls selectionFeedback() when running in Tauri", () => {
  markAsTauri()
  haptic('selection')
  expect(mockSelectionFeedback).toHaveBeenCalledTimes(1)
})

test.each([
  ['impactLight', 'light'],
  ['impactMedium', 'medium'],
] as const)("haptic('%s') calls impactFeedback('%s')", (kind, style) => {
  markAsTauri()
  haptic(kind)
  expect(mockImpactFeedback).toHaveBeenCalledWith(style)
})

test.each([
  ['notificationSuccess', 'success'],
  ['notificationError', 'error'],
] as const)("haptic('%s') calls notificationFeedback('%s')", (kind, type) => {
  markAsTauri()
  haptic(kind)
  expect(mockNotificationFeedback).toHaveBeenCalledWith(type)
})

test('swallows plugin errors (rejected promise) without throwing', async () => {
  markAsTauri()
  mockSelectionFeedback.mockRejectedValue(new Error('no haptics hardware'))

  expect(() => haptic('selection')).not.toThrow()

  // let the fire-and-forget promise settle before asserting nothing escaped
  await new Promise((resolve) => setTimeout(resolve, 0))
  expect(mockSelectionFeedback).toHaveBeenCalledTimes(1)
})

test('swallows synchronous plugin throws without throwing', () => {
  markAsTauri()
  mockImpactFeedback.mockImplementation(() => {
    throw new Error('synchronous failure')
  })

  expect(() => haptic('impactLight')).not.toThrow()
})
