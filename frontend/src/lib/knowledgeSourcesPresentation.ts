import type { DocsSpace } from './docsTypes';
import type { AgentKnowledgeSource, SupportContentSource } from './pmTypes';

export type KnowledgeSourceType = 'website' | 'helpin_docs' | 'file';
export type KnowledgeSourceStatus = 'queued' | 'running' | 'ready' | 'failed' | 'stale' | 'disabled';

export type KnowledgeSourceTypeOption = {
  id: KnowledgeSourceType;
  label: string;
  description: string;
  available: boolean;
};

export type KnowledgeSourceRow = {
  id: string;
  sourceId: string;
  knowledgeSourceId?: string;
  type: KnowledgeSourceType;
  typeLabel: string;
  scopeType?: 'space' | 'collection' | 'article';
  spaceId?: string;
  collectionId?: string | null;
  documentId?: string | null;
  name: string;
  description: string;
  status: KnowledgeSourceStatus;
  progress: number;
  countLabel: string;
  lastSyncAt?: string | null;
  syncStartedAt?: string | null;
  syncCompletedAt?: string | null;
  nextSyncAt?: string | null;
  indexedChunks?: number;
  error?: string | null;
  warning?: string | null;
};

type WebsiteSourceLike = Pick<
  SupportContentSource,
  | 'id'
  | 'name'
  | 'source_type'
  | 'start_url'
  | 'file_name'
  | 'file_size'
  | 'content_type'
  | 'sync_status'
  | 'sync_progress'
  | 'indexed_pages'
  | 'indexed_chunks'
  | 'next_sync_at'
  | 'last_sync_started_at'
  | 'last_sync_completed_at'
  | 'last_sync_error'
  | 'last_sync_warning'
>;

type DocsSpaceLike = Pick<DocsSpace, 'id' | 'name' | 'type'>;

type KnowledgeSourceLike = Pick<
  AgentKnowledgeSource,
  | 'id'
  | 'space_id'
  | 'scope_type'
  | 'collection_id'
  | 'document_id'
  | 'sync_status'
  | 'sync_progress'
  | 'indexed_documents'
  | 'indexed_chunks'
  | 'last_sync_started_at'
  | 'last_sync_completed_at'
  | 'last_sync_error'
  | 'space_type'
  | 'collection_name'
  | 'document_title'
>;

export const knowledgeSourceTypeOptions: KnowledgeSourceTypeOption[] = [
  {
    id: 'website',
    label: 'Website',
    description: 'Crawl public docs, help centers, or product pages from a URL.',
    available: true,
  },
  {
    id: 'helpin_docs',
    label: 'Helpin docs',
    description: 'Use a Helpin help-center space. Collection and article selection comes next.',
    available: true,
  },
  {
    id: 'file',
    label: 'PDF or file',
    description: 'Upload PDFs, Markdown, or text files as searchable AI sources.',
    available: true,
  },
];

export function buildKnowledgeSourceRows({
  websiteSources,
  docsSpaces,
  docsKnowledgeSources,
}: {
  websiteSources: WebsiteSourceLike[];
  docsSpaces: DocsSpaceLike[];
  docsKnowledgeSources: KnowledgeSourceLike[];
}): KnowledgeSourceRow[] {
  const rows: KnowledgeSourceRow[] = websiteSources.map((source) => {
    const isFile = source.source_type === 'file';
    return {
      id: `${isFile ? 'file' : 'website'}:${source.id}`,
      sourceId: source.id,
      type: isFile ? 'file' : 'website',
      typeLabel: isFile ? 'File' : 'Website',
      name: source.name,
      description: isFile
        ? fileSourceDescription(source.file_name, source.file_size)
        : source.start_url,
      status: source.sync_status,
      progress: source.sync_progress ?? 0,
      countLabel: `${source.indexed_pages ?? 0} ${(source.indexed_pages ?? 0) === 1 ? 'page' : 'pages'}`,
      ...knowledgeSyncTimes(source.last_sync_started_at, source.last_sync_completed_at),
      nextSyncAt: isFile ? null : source.next_sync_at ?? null,
      indexedChunks: source.indexed_chunks ?? 0,
      error: source.last_sync_error ?? null,
      warning: source.last_sync_warning ?? null,
    };
  });

  const spacesByID = new Map(docsSpaces.map((space) => [space.id, space]));

  for (const source of docsKnowledgeSources) {
    const space = spacesByID.get(source.space_id);
    if (!space) {
      continue;
    }
    rows.push({
      id: `helpin_docs:${source.id}`,
      sourceId: space.id,
      knowledgeSourceId: source.id,
      type: 'helpin_docs',
      typeLabel: docsSourceTypeLabel(source.scope_type),
      scopeType: source.scope_type ?? 'space',
      spaceId: source.space_id,
      collectionId: source.collection_id ?? null,
      documentId: source.document_id ?? null,
      name: docsSourceName(source, space.name),
      description: docsSourceDescription(source, space.name),
      status: source.sync_status,
      progress: source.sync_progress ?? 0,
      countLabel: `${source.indexed_documents ?? 0} ${(source.indexed_documents ?? 0) === 1 ? 'article' : 'articles'}`,
      ...knowledgeSyncTimes(source.last_sync_started_at, source.last_sync_completed_at),
      nextSyncAt: null,
      indexedChunks: source.indexed_chunks ?? 0,
      error: source.last_sync_error ?? null,
    });
  }

  return rows;
}

function knowledgeSyncTimes(started?: string | null, completed?: string | null) {
  const currentCompletion = completed && (!started || Date.parse(completed) >= Date.parse(started)) ? completed : null;
  return { lastSyncAt: currentCompletion ?? started ?? null, syncStartedAt: started ?? null, syncCompletedAt: currentCompletion };
}

export function knowledgeSourceCanBeManaged(
  source: Pick<KnowledgeSourceRow, 'type'>,
): boolean {
  return source.type === 'website';
}

function docsSourceTypeLabel(scopeType?: string) {
  switch (scopeType) {
    case 'collection':
      return 'Docs Collection';
    case 'article':
      return 'Docs Article';
    default:
      return 'Docs Space';
  }
}

function docsSourceName(source: KnowledgeSourceLike, spaceName: string) {
  switch (source.scope_type) {
    case 'collection':
      return source.collection_name || spaceName;
    case 'article':
      return source.document_title || source.collection_name || spaceName;
    default:
      return spaceName;
  }
}

function docsSourceDescription(source: KnowledgeSourceLike, spaceName: string) {
  switch (source.scope_type) {
    case 'collection':
      return spaceName;
    case 'article':
      return source.collection_name ? `${spaceName} / ${source.collection_name}` : spaceName;
    default:
      return source.space_type === 'internal' ? 'Internal Docs Space' : 'External Docs Space';
  }
}

function fileSourceDescription(fileName?: string | null, fileSize?: number) {
  const name = fileName?.trim() || 'Uploaded file';
  const size = formatFileSize(fileSize ?? 0);
  return size ? `${name} - ${size}` : name;
}

function formatFileSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '';
  }
  if (bytes < 1024) {
    return `${bytes}B`;
  }
  if (bytes < 1024 * 1024) {
    return `${Math.round(bytes / 1024)}KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(bytes >= 10 * 1024 * 1024 ? 0 : 1)}MB`;
}
