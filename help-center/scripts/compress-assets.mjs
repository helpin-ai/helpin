import { promisify } from 'node:util'
import { brotliCompress, constants, gzip } from 'node:zlib'
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const brotli = promisify(brotliCompress)
const gzipAsync = promisify(gzip)
const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const assetsDir = path.resolve(scriptDir, '..', 'dist', 'client', 'assets')
const compressibleExtensions = new Set(['.css', '.js', '.json'])
const minimumBytes = 1024

async function walk(directory) {
  const entries = await fs.readdir(directory, { withFileTypes: true })
  const files = await Promise.all(entries.map(async (entry) => {
    const fullPath = path.join(directory, entry.name)
    return entry.isDirectory() ? walk(fullPath) : [fullPath]
  }))
  return files.flat()
}

const files = await walk(assetsDir)
let compressed = 0

await Promise.all(files.map(async (filePath) => {
  if (!compressibleExtensions.has(path.extname(filePath))) return
  const input = await fs.readFile(filePath)
  if (input.byteLength < minimumBytes) return

  const [br, gz] = await Promise.all([
    brotli(input, {
      params: {
        [constants.BROTLI_PARAM_QUALITY]: 11,
      },
    }),
    gzipAsync(input, { level: 9 }),
  ])
  await Promise.all([
    fs.writeFile(`${filePath}.br`, br),
    fs.writeFile(`${filePath}.gz`, gz),
  ])
  compressed += 1
}))

console.log(`compressed ${compressed} help-center assets`)
