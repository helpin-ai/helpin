// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'

import { LoadingImage } from '../loading-image'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('LoadingImage', () => {
  it('shows a loader until the image load event fires', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <LoadingImage
          src="https://cdn.example.com/story.png"
          alt="Story image"
          containerClassName="h-20 w-20"
          className="h-20 w-20"
        />,
      )
    })

    expect(container.querySelector('[data-loading-image-spinner="true"]')).toBeTruthy()

    const image = container.querySelector('img')
    expect(image).toBeTruthy()

    act(() => {
      image?.dispatchEvent(new Event('load'))
    })

    expect(container.querySelector('[data-loading-image-spinner="true"]')).toBeNull()

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
