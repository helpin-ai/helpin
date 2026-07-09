import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('KnowledgeTab file upload control', () => {
  it('uses a button-style chooser instead of showing the native file input', () => {
    const source = readFileSync(resolve(__dirname, '../KnowledgeTab.tsx'), 'utf8');

    expect(source).toContain('Choose file');
    expect(source).toContain('className="sr-only"');
    expect(source).toContain('fileSourceInputRef.current?.click()');
  });

  it('shows the selected file name directly under the file label', () => {
    const source = readFileSync(resolve(__dirname, '../KnowledgeTab.tsx'), 'utf8');
    const labelIndex = source.indexOf('<Label htmlFor="knowledge-file-source">File</Label>');
    const nameIndex = source.indexOf('{selectedFileSource.name} - {formatFileSize(selectedFileSource.size)}');
    const buttonIndex = source.indexOf('{selectedFileSource ? \'Change file\' : \'Choose file\'}');

    expect(labelIndex).toBeGreaterThan(-1);
    expect(nameIndex).toBeGreaterThan(labelIndex);
    expect(buttonIndex).toBeGreaterThan(nameIndex);
  });

  it('uses tooltip-labeled icon actions for source edit, sync, and delete', () => {
    const source = readFileSync(resolve(__dirname, '../KnowledgeTab.tsx'), 'utf8');

    expect(source).toContain('<TooltipContent>Edit source</TooltipContent>');
    expect(source).toContain('<TooltipContent>Sync now</TooltipContent>');
    expect(source).toContain('<TooltipContent>Delete source</TooltipContent>');
    expect(source).not.toContain('title="Sync now"');
    expect(source).not.toContain('title="Remove"');
    expect(source).not.toContain('>\n                              Edit\n');
  });
});
