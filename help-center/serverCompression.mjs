import { promisify } from 'node:util'
import { brotliCompress, gzip } from 'node:zlib'

const brotli = promisify(brotliCompress)
const gzipAsync = promisify(gzip)

export function negotiateEncoding(acceptEncoding = '') {
  const accepted = new Map()
  for (const entry of acceptEncoding.toLowerCase().split(',')) {
    const [name, ...parameters] = entry.trim().split(';')
    if (!name) continue
    const qualityParameter = parameters.find((parameter) => parameter.trim().startsWith('q='))
    const quality = qualityParameter ? Number.parseFloat(qualityParameter.split('=')[1]) : 1
    accepted.set(name, Number.isFinite(quality) ? quality : 0)
  }

  const brotliQuality = accepted.get('br') ?? accepted.get('*') ?? 0
  const gzipQuality = accepted.get('gzip') ?? accepted.get('*') ?? 0
  if (brotliQuality > 0 && brotliQuality >= gzipQuality) return 'br'
  if (gzipQuality > 0) return 'gzip'
  return ''
}

export function appendVary(currentValue, name) {
  const values = String(currentValue || '')
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
  if (!values.some((value) => value.toLowerCase() === name.toLowerCase())) {
    values.push(name)
  }
  return values.join(', ')
}

export async function compressBody(body, encoding) {
  const input = Buffer.isBuffer(body) ? body : Buffer.from(body)
  if (encoding === 'br') return brotli(input)
  if (encoding === 'gzip') return gzipAsync(input)
  return input
}

export function compressedAssetPath(filePath, encoding, existsSync) {
  if (encoding === 'br' && existsSync(`${filePath}.br`)) {
    return { encoding, filePath: `${filePath}.br` }
  }
  if (encoding === 'gzip' && existsSync(`${filePath}.gz`)) {
    return { encoding, filePath: `${filePath}.gz` }
  }
  return { encoding: '', filePath }
}
