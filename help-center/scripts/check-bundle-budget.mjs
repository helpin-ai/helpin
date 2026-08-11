import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const assetsDir = path.resolve(scriptDir, '..', 'dist', 'client', 'assets')
const budgets = {
  css: 30 * 1024,
  main: 180 * 1024,
}

function largestMatching(pattern) {
  return fs.readdirSync(assetsDir)
    .filter((name) => pattern.test(name))
    .map((name) => ({ name, bytes: fs.statSync(path.join(assetsDir, name)).size }))
    .sort((a, b) => b.bytes - a.bytes)[0]
}

const checks = [
  ['main', largestMatching(/^main-.*\.js\.br$/)],
  ['css', largestMatching(/^app-.*\.css\.br$/)],
]
let failed = false

for (const [kind, asset] of checks) {
  if (!asset) {
    console.error(`missing Brotli ${kind} asset; run the production build first`)
    failed = true
    continue
  }
  const budget = budgets[kind]
  console.log(`${asset.name}: ${asset.bytes} bytes (budget ${budget})`)
  if (asset.bytes > budget) failed = true
}

if (failed) process.exitCode = 1
