export function isForwardedHeaderMarker(line: string): boolean {
  const normalized = line.trim().replace(/^>+\s*/, '').toLowerCase();
  return normalized.includes('forwarded message') || normalized.includes('begin forwarded message') || normalized.includes('original message');
}

export function hasForwardedHeaderMarker(content: string): boolean {
  return content.replace(/\r\n/g, '\n').split('\n').some(isForwardedHeaderMarker);
}

function isForwardedMetadataLine(line: string): boolean {
  return /^(from|date|sent|subject|to|cc|bcc):\s*/i.test(line.trim().replace(/^>+\s*/, ''));
}

export function cleanForwardedDisplayContent(content: string): string {
  const normalized = content.replace(/\r\n/g, '\n');
  const lines = normalized.split('\n');
  const markerIndex = lines.findIndex(isForwardedHeaderMarker);
  if (markerIndex < 0) return content;

  const note = lines.slice(0, markerIndex).join('\n').trim();
  let bodyStart = -1;
  let firstNonMetadata = -1;
  let sawMetadata = false;
  let headerEnded = false;

  for (let i = markerIndex + 1; i < lines.length; i += 1) {
    if (sawMetadata && lines[i].trim() === '') {
      headerEnded = true;
      continue;
    }
    if (headerEnded) {
      bodyStart = i;
      break;
    }
    const line = lines[i];
    if (isForwardedMetadataLine(line)) {
      sawMetadata = true;
      continue;
    }
    if (sawMetadata && firstNonMetadata < 0) {
      firstNonMetadata = i;
    }
  }

  if (bodyStart < 0) bodyStart = firstNonMetadata;

  if (bodyStart < 0) return note || content;

  const bodyLines: string[] = [];
  for (let i = bodyStart; i < lines.length; i += 1) {
    if (i !== bodyStart && isForwardedHeaderMarker(lines[i])) {
      break;
    }
    bodyLines.push(lines[i]);
  }

  const body = bodyLines.join('\n').trim();
  return [note, body].filter(Boolean).join('\n\n') || content;
}
