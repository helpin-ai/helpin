import { describe, expect, it } from 'vitest'
import {
  appendVary,
  compressedAssetPath,
  compressBody,
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
})
