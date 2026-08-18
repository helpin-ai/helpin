import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const html = readFileSync(new URL('../../../index.html', import.meta.url), 'utf8')

function expectMeta(attribute: 'name' | 'property', key: string, content: string) {
  const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const escapedContent = content.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  expect(html).toMatch(new RegExp(`<meta ${attribute}="${escapedKey}" content="${escapedContent}"\\s*/?>`))
}

describe('app HTML metadata', () => {
  it('provides a complete crawler-safe default social preview', () => {
    expect(html).toContain('<!-- helpin-meta:start -->')
    expect(html).toContain('<!-- helpin-meta:end -->')
    expect(html).toContain("<title>Helpin — Your Team's Work, Connected</title>")

    expectMeta('name', 'application-name', 'Helpin')
    expectMeta('name', 'description', 'Projects, support, sales, docs, and AI agents in one connected workspace.')
    expectMeta('name', 'robots', 'noindex, nofollow, noarchive, nosnippet')
    expectMeta('name', 'googlebot', 'noindex, nofollow, noarchive, nosnippet')
    expectMeta('property', 'og:title', "Helpin — Your Team's Work, Connected")
    expectMeta('property', 'og:description', 'Projects, support, sales, docs, and AI agents in one connected workspace.')
    expectMeta('property', 'og:type', 'website')
    expectMeta('property', 'og:url', 'https://app.helpin.ai/')
    expectMeta('property', 'og:site_name', 'Helpin')
    expectMeta('property', 'og:locale', 'en_US')
    expectMeta('property', 'og:image', 'https://app.helpin.ai/og/helpin-app.png')
    expectMeta('property', 'og:image:secure_url', 'https://app.helpin.ai/og/helpin-app.png')
    expectMeta('property', 'og:image:type', 'image/png')
    expectMeta('property', 'og:image:width', '1200')
    expectMeta('property', 'og:image:height', '630')
    expectMeta('property', 'og:image:alt', "Helpin connects your team's work in one workspace")
    expectMeta('name', 'twitter:card', 'summary_large_image')
    expectMeta('name', 'twitter:title', "Helpin — Your Team's Work, Connected")
    expectMeta('name', 'twitter:description', 'Projects, support, sales, docs, and AI agents in one connected workspace.')
    expectMeta('name', 'twitter:image', 'https://app.helpin.ai/og/helpin-app.png')
    expectMeta('name', 'twitter:image:alt', "Helpin connects your team's work in one workspace")

    expect(html).not.toMatch(/<link[^>]+rel="canonical"/i)
    expect(html).not.toMatch(/<meta[^>]+name="keywords"/i)
  })
})
