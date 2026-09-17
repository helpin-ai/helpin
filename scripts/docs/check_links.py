#!/usr/bin/env python3
"""Check local Markdown links in maintained documentation, without network access."""
import argparse
import html
import json
from pathlib import Path
import re
import sys
import unicodedata
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[2]


def prose(text):
    """Remove fenced examples and inline code while preserving line numbers."""
    result = []
    fence = None
    for line in text.splitlines(keepends=True):
        marker = re.match(r'^\s{0,3}(`{3,}|~{3,})', line)
        if fence:
            if re.match(r'^\s{0,3}' + re.escape(fence[0]) + '{' + str(len(fence)) + r',}\s*$', line):
                fence = None
            result.append('\n' if line.endswith('\n') else '')
        elif marker:
            fence = marker[1]
            result.append('\n' if line.endswith('\n') else '')
        else:
            result.append(line)
    text = ''.join(result)
    # Comments often contain sample links or instructions rather than prose.
    text = re.sub(r'<!--[\s\S]*?-->', lambda m: '\n' * m[0].count('\n'), text)
    return re.sub(r'(`+)([^`]*?)\1', lambda m: ' ' * len(m[0]), text)


def anchors(text):
    # Preserve inline-code content in heading labels before removing examples.
    text = re.sub(r'`([^`\n]+)`', r'\1', text)
    clean = prose(text)
    found = set(re.findall(r'<[^>]+\b(?:id|name)=["\']([^"\']+)["\']', clean))
    used = set()
    lines = clean.splitlines()
    for index, line in enumerate(lines):
        atx = re.match(r'^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$', line)
        setext = index + 1 < len(lines) and re.fullmatch(r'\s{0,3}(?:=+|-+)\s*', lines[index + 1])
        if not atx and not (line.strip() and setext):
            continue
        label = atx[1] if atx else line.strip()
        label = re.sub(r'!?\[([^]]+)\]\([^)]*\)', r'\1', label)
        label = html.unescape(re.sub(r'<[^>]*>', '', label)).lower()
        slug = ''.join(c for c in label if c in '-_ ' or unicodedata.category(c)[0] in 'LN').replace(' ', '-')
        base, counter = slug, 0
        while slug in used:
            counter += 1
            slug = f'{base}-{counter}'
        used.add(slug)
        found.add(slug)
    return found


def links(text):
    clean = prose(text)
    # Link destinations may be angle-bracketed or contain balanced parentheses.
    destination = r'<[^>\n]+>|(?:[^\s()\\]|\\.|\([^\n]*?\))+'
    definitions = {}
    definition_spans = []
    for match in re.finditer(r'^\s{0,3}\[([^]]+)\]:\s*(' + destination + ')', clean, re.M):
        definitions[' '.join(match[1].split()).casefold()] = match[2]
        definition_spans.append(match.span())
        yield match.start(), match[2], None
    inline_spans = []
    for match in re.finditer(r'!?\[[^]\n]*\]\(\s*(' + destination + r')(?:\s+["\'][^\n]*?["\'])?\s*\)', clean):
        inline_spans.append(match.span())
        yield match.start(), match[1], None
    for match in re.finditer(r'!?\[([^]\n]+)\]\[([^]\n]*)\]', clean):
        if any(start <= match.start() < end for start, end in definition_spans + inline_spans):
            continue
        key = ' '.join((match[2] or match[1]).split()).casefold()
        if key not in definitions:
            yield match.start(), '', f'undefined link reference: {key}'
    # Defined shortcut references were already checked at their definition.
    # Undefined shortcuts are ordinary bracketed text in Markdown, not links.


def check(root, files):
    root = root.resolve()
    errors = []
    count = 0
    for name in files:
        source = root / name
        if not source.is_file():
            errors.append(f'{name}: document does not exist')
            continue
        text = source.read_text(encoding='utf-8')
        clean = prose(text)
        for offset, destination, error in links(text):
            line = clean[:offset].count('\n') + 1
            prefix = f'{name}:{line}'
            if error:
                errors.append(f'{prefix}: {error}')
                continue
            destination = re.sub(r'\\(.)', r'\1', html.unescape(destination.strip('<>')))
            try:
                url = urlsplit(destination)
            except ValueError:
                errors.append(f'{prefix}: invalid destination {destination}')
                continue
            if url.scheme or url.netloc:
                continue
            count += 1
            path = unquote(url.path)
            target = (source.parent / path).resolve() if path else source.resolve()
            if not target.is_relative_to(root):
                errors.append(f'{prefix}: link leaves repository: {destination}')
            elif not target.exists():
                errors.append(f'{prefix}: missing target: {destination}')
            elif url.fragment and target.is_file() and target.suffix.lower() == '.md':
                fragment = unquote(url.fragment)
                if fragment not in anchors(target.read_text(encoding='utf-8')):
                    errors.append(f'{prefix}: missing heading or anchor: {destination}')
    return count, errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('files', nargs='*', help='Repository-relative documents; defaults to maintained-docs.json')
    args = parser.parse_args()
    files = args.files or json.loads((ROOT / 'scripts/docs/maintained-docs.json').read_text())
    count, errors = check(ROOT, files)
    for error in errors:
        print(error, file=sys.stderr)
    print(f'Checked {len(files)} documents and {count} local links; {len(errors)} errors.')
    return bool(errors)


if __name__ == '__main__':
    sys.exit(main())
