import { describe, expect, it } from 'vitest';

import {
  buildKnowledgeSourceRows,
  knowledgeSourceTypeOptions,
  knowledgeSourceCanBeManaged,
} from '../knowledgeSourcesPresentation';

describe('knowledgeSourcesPresentation', () => {
  it('builds one unified source list from websites and selected Helpin docs spaces', () => {
    const rows = buildKnowledgeSourceRows({
      websiteSources: [
        {
          id: 'site-1',
          name: 'Docs site',
          source_type: 'website',
          start_url: 'https://docs.example.com',
          file_name: null,
          file_size: 0,
          content_type: null,
          sync_status: 'ready',
          sync_progress: 100,
          indexed_pages: 12,
          last_sync_warning: '2 URLs skipped because the site disallows crawling.',
          indexed_chunks: 45,
          last_sync_completed_at: '2026-07-08T10:00:00Z',
        },
        {
          id: 'file-1',
          name: 'Product guide',
          source_type: 'file',
          start_url: 'file://guide.pdf',
          file_name: 'guide.pdf',
          file_size: 2048,
          content_type: 'application/pdf',
          sync_status: 'queued',
          sync_progress: 0,
          indexed_pages: 0,
          indexed_chunks: 0,
          last_sync_completed_at: null,
        },
      ],
      docsSpaces: [
        { id: 'space-1', name: 'Help Center', type: 'external_capable' },
        { id: 'space-2', name: 'Internal Notes', type: 'internal' },
      ],
      docsKnowledgeSources: [
        {
          id: 'ks-1',
          space_id: 'space-1',
          scope_type: 'space',
          sync_status: 'running',
          sync_progress: 40,
          indexed_documents: 5,
          indexed_chunks: 28,
          last_sync_started_at: '2026-07-08T10:05:00Z',
        },
      ],
    });

    expect(rows).toEqual([
      expect.objectContaining({
        id: 'website:site-1',
        sourceId: 'site-1',
        type: 'website',
        typeLabel: 'Website',
        name: 'Docs site',
        status: 'ready',
        countLabel: '12 pages',
        warning: '2 URLs skipped because the site disallows crawling.',
        error: null,
      }),
      expect.objectContaining({
        id: 'file:file-1',
        sourceId: 'file-1',
        type: 'file',
        typeLabel: 'File',
        name: 'Product guide',
        description: 'guide.pdf - 2KB',
        status: 'queued',
        countLabel: '0 pages',
      }),
      expect.objectContaining({
        id: 'helpin_docs:ks-1',
        sourceId: 'space-1',
        type: 'helpin_docs',
        typeLabel: 'Docs Space',
        name: 'Help Center',
        description: 'External Docs Space',
        status: 'running',
        countLabel: '5 articles',
      }),
    ]);
  });

  it('exposes source type choices with PDFs available', () => {
    expect(knowledgeSourceTypeOptions).toEqual([
      expect.objectContaining({ id: 'website', label: 'Website', available: true }),
      expect.objectContaining({ id: 'helpin_docs', label: 'Helpin docs', available: true }),
      expect.objectContaining({ id: 'file', label: 'PDF or file', available: true }),
    ]);
  });

  it('only exposes manage/edit for website sources', () => {
    expect(knowledgeSourceCanBeManaged({ type: 'website' })).toBe(true);
    expect(knowledgeSourceCanBeManaged({ type: 'helpin_docs' })).toBe(false);
    expect(knowledgeSourceCanBeManaged({ type: 'file' })).toBe(false);
  });
});
