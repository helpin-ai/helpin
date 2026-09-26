// Run from any directory: node website/scripts/build-brand-kit.mjs
// Export existing vector artwork without redrawing it or depending on installed fonts.
import { readFile, writeFile, mkdir, copyFile } from 'node:fs/promises';
import sharp from 'sharp';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const website = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const brand = resolve(website, 'public/brand');
const kit = resolve(brand, 'kit');
await mkdir(kit, { recursive: true });
const palette = JSON.parse(await readFile(resolve(kit, 'helpin-palette.json'), 'utf8'));
const properties = palette.colors.map(({ name, hex }) => `  --helpin-${name.toLowerCase().replaceAll(' ', '-')}: ${hex};`);
await writeFile(resolve(kit, 'helpin-palette.css'), `/* Helpin website palette */\n:root {\n${properties.join('\n')}\n}\n`);
for (const color of ['ink', 'white']) {
  const source = resolve(brand, `helpin-icon-${color}.svg`);
  await copyFile(source, resolve(kit, `helpin-symbol-${color}.svg`));
  await sharp(source, { density: 400 }).resize(512, 512).png().toFile(resolve(kit, `helpin-symbol-${color}-512.png`));
}
// A portable archive with fixed entry timestamps; Python's zipfile avoids another dependency.
execFileSync('python3', ['-c', `
from pathlib import Path
from zipfile import ZipFile, ZipInfo, ZIP_DEFLATED
kit = Path(__import__('sys').argv[1])
names = ['helpin-symbol-ink.svg', 'helpin-symbol-white.svg', 'helpin-symbol-ink-512.png', 'helpin-symbol-white-512.png', 'helpin-palette.json', 'helpin-palette.css']
with ZipFile(kit / 'helpin-brand-kit.zip', 'w') as archive:
    for name in names:
        entry = ZipInfo(name, (2026, 1, 1, 0, 0, 0))
        entry.compress_type = ZIP_DEFLATED
        entry.external_attr = 0o100644 << 16
        archive.writestr(entry, (kit / name).read_bytes())
`, kit]);
console.log('Exported Helpin symbols, palette, and brand kit.');
