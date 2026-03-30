import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { LocaleSwitcher } from '@/components/layout/LocaleSwitcher'

describe('LocaleSwitcher', () => {
  it('switches to the matching locale path when translation exists', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()

    render(
      <LocaleSwitcher
        currentLocale="en"
        options={[
          { code: 'en', label: 'English', href: '/en/basics/start-here', active: true },
          { code: 'fr', label: 'Francais', href: '/fr/bases/bonjour-fr', active: false },
        ]}
        onSelect={onSelect}
      />,
    )

    await user.click(screen.getByRole('button', { name: /english/i }))
    await user.click(screen.getByRole('menuitem', { name: /francais/i }))

    expect(onSelect).toHaveBeenCalledWith('/fr/bases/bonjour-fr')
  })
})
