import { describe, expect, it } from 'vitest'
import {
  appendVary,
  compressedAssetPath,
  compressBody,
  isCompressibleContentType,
  negotiateEncoding,
} from './serverCompression.mjs'

describe('server compression', () => {
  it('prefers Brotli and honors disabled encodings', () => {
    expect(negotiateEncoding('gzip, br')).toBe('br')
    expect(negotiateEncoding('br;q=0, gzip;q=0.8')).toBe('gzip')
    expect(negotiateEncoding('identity')).toBe('')
  })

  it('adds Accept-Encoding to Vary once', () => {
    expect(appendVary('', 'Accept-Encoding')).toBe('Accept-Encoding')
    expect(appendVary('Origin, Accept-Encoding', 'accept-encoding'))
      .toBe('Origin, Accept-Encoding')
  })

  it('selects an existing precompressed asset', () => {
    const exists = (path) => path.endsWith('.br')
    expect(compressedAssetPath('/assets/main.js', 'br', exists)).toEqual({
      encoding: 'br',
      filePath: '/assets/main.js.br',
    })
  })

  it('compresses response bodies', async () => {
    const input = 'help center '.repeat(200)
    const compressed = await compressBody(input, 'gzip')
    expect(compressed.byteLength).toBeLessThan(Buffer.byteLength(input))
  })

  it('compresses Brotli bodies that round-trip', async () => {
    const { brotliDecompressSync } = await import('node:zlib')
    const input = JSON.stringify({ items: Array.from({ length: 200 }, (_, i) => ({ i })) })
    const compressed = await compressBody(input, 'br')
    expect(compressed.byteLength).toBeLessThan(Buffer.byteLength(input))
    expect(brotliDecompressSync(compressed).toString()).toBe(input)
  })

  it('compresses JSON and text but not binary types', () => {
    expect(isCompressibleContentType('application/json; charset=utf-8')).toBe(true)
    expect(isCompressibleContentType('application/problem+json')).toBe(true)
    expect(isCompressibleContentType('text/html')).toBe(true)
    expect(isCompressibleContentType('image/png')).toBe(false)
    expect(isCompressibleContentType('')).toBe(false)
  })
})
