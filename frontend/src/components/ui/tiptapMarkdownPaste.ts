function commonLeadingIndent(lines: string[]) {
  let common: number | null = null;
  for (const line of lines) {
    if (line.trim() === '') continue;
    const indent = line.match(/^[ \t]*/)?.[0] ?? '';
    const width = indent.replace(/\t/g, '    ').length;
    common = common === null ? width : Math.min(common, width);
  }
  return common ?? 0;
}

function removeIndent(line: string, width: number) {
  let remaining = width;
  let index = 0;
  while (index < line.length && remaining > 0) {
    const char = line[index];
    if (char === ' ') {
      remaining -= 1;
      index += 1;
      continue;
    }
    if (char === '\t') {
      remaining -= 4;
      index += 1;
      continue;
    }
    break;
  }
  return line.slice(index);
}

function looksLikeMarkdown(value: string) {
  const lines = value.split(/\r?\n/);
  return lines.some((line) => /^(#{1,6}\s+\S|[-+*]\s+\S|\d+[.)]\s+\S|>\s+\S)/.test(line))
    || /\[[^\]\n]+\]\([^)]+\)/.test(value)
    || /(^|[^*])\*\*[^*\n]+\*\*/.test(value)
    || /(^|[^_])__[^_\n]+__/.test(value);
}

export function normalizePastedMarkdownText(text: string) {
  const normalizedNewlines = text.replace(/\r\n?/g, '\n');
  const lines = normalizedNewlines.split('\n');
  const indent = commonLeadingIndent(lines);
  if (indent < 4) return text;

  const dedented = lines.map((line) => removeIndent(line, indent)).join('\n');
  return looksLikeMarkdown(dedented) ? dedented : text;
}
