// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const testState = vi.hoisted(() => ({
  spaces: [
    { id: 'space-1', name: 'General', icon: '', type: 'internal' },
    { id: 'space-2', name: 'Support', icon: '', type: 'internal' },
  ],
  collections: [],
  mutateAsync: vi.fn(),
}))

vi.mock('@/hooks/queries', () => ({
  useCreateDocsDocument: () => ({
    mutateAsync: testState.mutateAsync,
    isPending: false,
  }),
  useDocsSpaces: () => ({
    data: testState.spaces,
  }),
  useDocsCollections: () => ({
    data: testState.collections,
  }),
}))

vi.mock('@/components/ui/dialog', () => ({
  Dialog: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogDescription: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock('@/components/ui/button', () => ({
  Button: ({
    children,
    onClick,
    type = 'button',
    disabled,
  }: {
    children: React.ReactNode
    onClick?: () => void
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
  }) => (
    <button type={type} onClick={onClick} disabled={disabled}>
      {children}
    </button>
  ),
}))

vi.mock('@/components/ui/input', () => ({
  Input: ({
    value,
    onChange,
    id,
    placeholder,
    autoFocus,
  }: {
    value?: string
    onChange?: (event: React.ChangeEvent<HTMLInputElement>) => void
    id?: string
    placeholder?: string
    autoFocus?: boolean
  }) => (
    <input
      id={id}
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      autoFocus={autoFocus}
    />
  ),
}))

vi.mock('@/components/ui/label', () => ({
  Label: ({
    children,
    htmlFor,
  }: {
    children: React.ReactNode
    htmlFor?: string
  }) => <label htmlFor={htmlFor}>{children}</label>,
}))

vi.mock('@/components/ui/select', () => ({
  Select: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectItem: ({ children }: { children: React.ReactNode; value: string }) => <div>{children}</div>,
  SelectTrigger: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectValue: ({ placeholder }: { placeholder?: string }) => <span>{placeholder}</span>,
}))

vi.mock('@/components/ui/icon-picker', () => ({
  ICON_MAP: {},
}))

vi.mock('lucide-react', () => ({
  FolderOpen: () => null,
}))

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

import { CreateDocumentDialog } from '../CreateDocumentDialog'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('CreateDocumentDialog', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    testState.mutateAsync.mockReset()
    testState.mutateAsync.mockResolvedValue({ id: 'doc-1' })
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('resets to the provided default space when reopened from a selected space context', async () => {
    const onOpenChange = vi.fn()

    await act(async () => {
      root.render(
        <CreateDocumentDialog
          wsId="ws-1"
          open
          onOpenChange={onOpenChange}
        />,
      )
    })

    await act(async () => {
      root.render(
        <CreateDocumentDialog
          wsId="ws-1"
          open={false}
          onOpenChange={onOpenChange}
        />,
      )
    })

    await act(async () => {
      root.render(
        <CreateDocumentDialog
          wsId="ws-1"
          open
          onOpenChange={onOpenChange}
          defaultSpaceId="space-2"
        />,
      )
    })

    const titleInput = container.querySelector('input')
    const form = container.querySelector('form')

    await act(async () => {
      if (titleInput instanceof HTMLInputElement) {
        const valueSetter = Object.getOwnPropertyDescriptor(
          HTMLInputElement.prototype,
          'value',
        )?.set
        valueSetter?.call(titleInput, 'Runbook')
        titleInput.dispatchEvent(new Event('input', { bubbles: true }))
        titleInput.dispatchEvent(new Event('change', { bubbles: true }))
      }
    })

    await act(async () => {
      form?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })

    expect(testState.mutateAsync).toHaveBeenCalledWith({
      title: 'Runbook',
      space_id: 'space-2',
      collection_id: undefined,
    })
  })
})
