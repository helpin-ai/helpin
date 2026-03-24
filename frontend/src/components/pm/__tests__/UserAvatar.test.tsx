// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { UserAvatar } from '../UserAvatar'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('UserAvatar', () => {
  const OriginalImage = window.Image

  beforeEach(() => {
    class LoadedImage {
      complete = true
      naturalWidth = 1
      src = ''
      referrerPolicy = ''
      crossOrigin: string | null = null

      addEventListener() {}
      removeEventListener() {}
    }

    window.Image = LoadedImage as typeof window.Image
  })

  afterEach(() => {
    window.Image = OriginalImage
  })

  it('shows fallback initials immediately when switching from an uploaded avatar to a default avatar', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <UserAvatar
          name="Alice Johnson"
          avatarUrl="https://cdn.example.com/alice.png"
          className="h-5 w-5"
        />,
      )
    })

    expect(container.querySelector('[data-slot=\"avatar-image\"]')).toBeTruthy()

    act(() => {
      root.render(
        <UserAvatar
          name="Bob Smith"
          avatarUrl={undefined}
          className="h-5 w-5"
        />,
      )
    })

    expect(container.textContent).toContain('BS')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
