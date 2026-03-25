// @vitest-environment jsdom
import { act } from 'react'
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const saveMutation = vi.fn()

vi.mock('@/components/ui/card', () => ({
  Card: ({ children }: { children: ReactNode }) => <section>{children}</section>,
  CardHeader: ({ children }: { children: ReactNode }) => <header>{children}</header>,
  CardTitle: ({ children }: { children: ReactNode }) => <h2>{children}</h2>,
  CardDescription: ({ children }: { children: ReactNode }) => <p>{children}</p>,
  CardContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}))

vi.mock('@/components/ui/button', () => ({
  Button: ({ children, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) => <button {...props}>{children}</button>,
}))

vi.mock('@/components/ui/label', () => ({
  Label: ({ children, htmlFor }: { children: ReactNode; htmlFor?: string }) => <label htmlFor={htmlFor}>{children}</label>,
}))

vi.mock('@/components/ui/switch', () => ({
  Switch: ({ checked, onCheckedChange, id }: { checked: boolean; onCheckedChange: (value: boolean) => void; id?: string }) => (
    <input
      id={id}
      aria-label={id}
      type="checkbox"
      checked={checked}
      onChange={(event) => onCheckedChange(event.currentTarget.checked)}
    />
  ),
}))

vi.mock('@/components/ui/checkbox', () => ({
  Checkbox: ({ checked, onCheckedChange, id }: { checked: boolean; onCheckedChange: (value: boolean) => void; id?: string }) => (
    <input
      id={id}
      aria-label={id}
      type="checkbox"
      checked={checked}
      onChange={(event) => onCheckedChange(event.currentTarget.checked)}
    />
  ),
}))

vi.mock('@/components/ui/select', () => ({
  Select: ({ value, onValueChange, children }: { value: string; onValueChange: (value: string) => void; children: ReactNode }) => (
    <div data-select-value={value} data-on-change={onValueChange}>
      {children}
    </div>
  ),
  SelectTrigger: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  SelectValue: () => null,
  SelectContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  SelectItem: ({ children, value }: { children: ReactNode; value: string }) => <option value={value}>{children}</option>,
}))

vi.mock('@/components/ui/input', () => ({
  Input: (props: InputHTMLAttributes<HTMLInputElement>) => <input {...props} />,
}))

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

import { HelpcenterLocalesCard } from '../HelpcenterLocalesCard'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('HelpcenterLocalesCard', () => {
  afterEach(() => {
    saveMutation.mockReset()
  })

  it('saves default locale and enabled locale settings', async () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <HelpcenterLocalesCard
          config={{
            default_locale: 'en',
            enabled_locales: ['en', 'fr'],
            show_language_switcher: true,
            fallback_to_default_locale: true,
          }}
          isSaving={false}
          onSave={saveMutation}
        />,
      )
    })

    const enableGerman = container.querySelector('input[id="locale-de"]') as HTMLInputElement | null
    expect(enableGerman).not.toBeNull()
    act(() => {
      enableGerman?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Save locales'))
    expect(saveButton).toBeTruthy()
    act(() => {
      saveButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(saveMutation).toHaveBeenCalledWith({
      default_locale: 'en',
      enabled_locales: ['en', 'fr', 'de'],
      show_language_switcher: true,
      fallback_to_default_locale: true,
    })

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
