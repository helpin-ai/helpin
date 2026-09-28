// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { PortalMessage } from '../PortalConversation'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root
let container: HTMLDivElement
beforeEach(() => {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
})

const render = (message: Parameters<typeof PortalMessage>[0]['message']) =>
  act(async () => root.render(<ol><PortalMessage message={message} supportName="Acme Support" /></ol>))

describe('PortalMessage', () => {
  it('renders support Markdown with safe links and no raw HTML', async () => {
    await render({
      id: 'm1', sender_type: 'ai', sender_name: 'Nova', created_at: new Date().toISOString(),
      content: '**What it does**\n\n- Syncs campaigns\n- Tracks `creative`\n\nGuide: [docs](https://example.com/docs) <script>alert(1)</script> [bad](javascript:alert(1))',
    })
    expect(container.querySelector('strong')?.textContent).toBe('What it does')
    expect(container.querySelectorAll('li li, ul li')).toHaveLength(2)
    expect(container.querySelector('code')?.textContent).toBe('creative')
    const link = container.querySelector<HTMLAnchorElement>('a[href="https://example.com/docs"]')
    expect(link?.target).toBe('_blank')
    expect(link?.rel).toContain('noopener')
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('a[href^="javascript:"]')).toBeNull()
    expect(container.textContent).not.toContain('**')
    expect(container.textContent).toContain('Nova')
    expect(container.textContent).toContain('AI')
  })

  it('shows customer text exactly as written on the right', async () => {
    await render({ id: 'm2', sender_type: 'customer', created_at: new Date().toISOString(), content: 'Line one\n**not bold**' })
    const bubble = container.querySelector('p.whitespace-pre-wrap')
    expect(bubble?.textContent).toBe('Line one\n**not bold**')
    expect(container.querySelector('strong')).toBeNull()
    expect(container.querySelector('li')?.className).toContain('justify-end')
    expect(container.textContent).toContain('You')
  })

  it('shows the agent name and avatar', async () => {
    await render({ id: 'm3', sender_type: 'user', sender_name: 'Priya', sender_avatar: 'https://avatars.example/priya.png', created_at: new Date().toISOString(), content: 'Hi' })
    expect(container.querySelector('img')?.getAttribute('src')).toBe('https://avatars.example/priya.png')
    expect(container.textContent).toContain('Priya')
    expect(container.textContent).not.toContain('AI')
  })
  it('draws a teammate\'s generated avatar, as the app does', async () => {
    await render({
      id: 'm9', sender_type: 'user', sender_name: 'Priya', content: 'Hi', created_at: new Date().toISOString(),
      sender_avatar_style: { style: 'micah', seed: 'seed-1', background_mode: 'color', background_color: '#f97316' },
    })
    // The avatar library loads on demand.
    await act(async () => { await import('@/lib/teamMemberAvatar') })
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)) })
    const img = container.querySelector('img')
    expect(img?.getAttribute('src')).toMatch(/^data:image\/svg\+xml/)
    expect(container.textContent).not.toContain('PPriya')
  })
})
