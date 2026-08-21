import { fireEvent, render, screen } from '@testing-library/react'
import { afterAll, beforeAll, vi } from 'vitest'
import type { SupportAttachmentPayload } from '@helpin-ai/support-core'
import { ImageViewer } from '../image-viewer'

vi.mock('@tauri-apps/plugin-opener', () => ({ openUrl: vi.fn() }))

class TestPointerEvent extends MouseEvent {
  pointerId: number

  constructor(type: string, init: MouseEventInit & { pointerId?: number } = {}) {
    super(type, init)
    this.pointerId = init.pointerId ?? 0
  }
}

beforeAll(() => vi.stubGlobal('PointerEvent', TestPointerEvent))
afterAll(() => vi.unstubAllGlobals())

const images: SupportAttachmentPayload[] = [
  {
    id: 'image-1',
    file_key: 'one.png',
    file_name: 'one.png',
    file_type: 'image/png',
    file_size: 100,
    url: 'https://cdn.example.com/one.png',
  },
  {
    id: 'image-2',
    file_key: 'two.png',
    file_name: 'two.png',
    file_type: 'image/png',
    file_size: 200,
    url: 'https://cdn.example.com/two.png',
  },
]

test('navigates between images without leaving the viewer', () => {
  render(<ImageViewer images={images} initialIndex={0} open onOpenChange={vi.fn()} />)

  expect(screen.getByText('1 of 2')).toBeDefined()
  expect(screen.getByAltText('one.png').getAttribute('src')).toBe(images[0].url)

  fireEvent.click(screen.getByRole('button', { name: 'Next image' }))

  expect(screen.getByText('2 of 2')).toBeDefined()
  expect(screen.getByAltText('two.png').getAttribute('src')).toBe(images[1].url)
})

test('closes back to the conversation', () => {
  const onOpenChange = vi.fn()
  render(<ImageViewer images={images} initialIndex={0} open onOpenChange={onOpenChange} />)

  fireEvent.click(screen.getByRole('button', { name: 'Close image viewer' }))

  expect(onOpenChange).toHaveBeenCalledWith(false)
})

test('supports swipe navigation and double-tap zoom on the image stage', () => {
  render(<ImageViewer images={images} initialIndex={0} open onOpenChange={vi.fn()} />)
  const stage = screen.getByTestId('image-viewer-stage')
  Object.defineProperty(stage, 'setPointerCapture', { configurable: true, value: vi.fn() })

  fireEvent.pointerDown(stage, { pointerId: 1, clientX: 180, clientY: 100 })
  fireEvent.pointerMove(stage, { pointerId: 1, clientX: 80, clientY: 100 })
  fireEvent.pointerUp(stage, { pointerId: 1, clientX: 80, clientY: 100 })
  expect(screen.getByText('2 of 2')).toBeDefined()

  const currentStage = screen.getByTestId('image-viewer-stage')
  fireEvent.pointerDown(currentStage, { pointerId: 2, clientX: 100, clientY: 100 })
  fireEvent.pointerUp(currentStage, { pointerId: 2, clientX: 100, clientY: 100 })
  fireEvent.pointerDown(currentStage, { pointerId: 3, clientX: 100, clientY: 100 })
  fireEvent.pointerUp(currentStage, { pointerId: 3, clientX: 100, clientY: 100 })

  expect(screen.getByAltText('two.png').style.transform).toContain('scale(2.5)')
})
