export type HelpinReferenceType =
  | 'tasks'
  | 'epics'
  | 'sprints'
  | 'objectives'
  | 'documents'
  | 'support-conversations'
  | 'deals'
  | 'contacts'
  | 'companies'
  | 'agent-runs'
  | 'artifacts';

export type HelpinReference = {
  type: HelpinReferenceType;
  id: string;
};

const INTERNAL_REFERENCE_PATH = '/__helpin/reference/';
const CANONICAL_REFERENCE_RE = /^helpin:\/\/([a-z_-]+)\/([^/?#]+)$/i;
const LEGACY_ARTIFACT_RE = /^helpin-artifact:\/\/([^/?#]+)$/i;

const referenceTypeAliases: Record<string, HelpinReferenceType> = {
  task: 'tasks',
  tasks: 'tasks',
  epic: 'epics',
  epics: 'epics',
  sprint: 'sprints',
  sprints: 'sprints',
  objective: 'objectives',
  objectives: 'objectives',
  document: 'documents',
  documents: 'documents',
  support_conversation: 'support-conversations',
  'support-conversations': 'support-conversations',
  deal: 'deals',
  deals: 'deals',
  contact: 'contacts',
  contacts: 'contacts',
  company: 'companies',
  companies: 'companies',
  agent_run: 'agent-runs',
  'agent-runs': 'agent-runs',
  artifact: 'artifacts',
  artifacts: 'artifacts',
};

/** Parses canonical and supported legacy Helpin resource references. */
export function parseHelpinReference(value: string | undefined): HelpinReference | null {
  const raw = value?.trim() ?? '';
  const legacyArtifact = raw.match(LEGACY_ARTIFACT_RE);
  if (legacyArtifact) return decodedReference('artifacts', legacyArtifact[1]);

  const match = raw.match(CANONICAL_REFERENCE_RE);
  if (!match) return null;
  const type = referenceTypeAliases[match[1].toLowerCase()];
  if (!type) return null;
  return decodedReference(type, match[2]);
}

/** Converts a Helpin reference into a safe relative href for markdown sanitizers. */
export function helpinReferenceMarker(value: string): string | null {
  if (!parseHelpinReference(value)) return null;
  return `${INTERNAL_REFERENCE_PATH}${encodeURIComponent(value.trim())}`;
}

/** Recovers a Helpin reference from the safe markdown href marker. */
export function parseHelpinReferenceMarker(value: string | undefined): HelpinReference | null {
  if (!value?.startsWith(INTERNAL_REFERENCE_PATH)) return null;
  try {
    return parseHelpinReference(decodeURIComponent(value.slice(INTERNAL_REFERENCE_PATH.length)));
  } catch {
    return null;
  }
}

/** Builds the current-workspace route for a navigable Helpin reference. */
export function helpinReferenceRoute(reference: HelpinReference, workspaceSlug: string): string | null {
  const slug = encodeURIComponent(workspaceSlug.trim());
  const id = encodeURIComponent(reference.id);
  if (!slug) return null;
  switch (reference.type) {
    case 'tasks':
      return `/w/${slug}/pm/tasks/${id}`;
    case 'epics':
      return `/w/${slug}/pm/epics/${id}`;
    case 'sprints':
      return `/w/${slug}/pm/sprints/${id}`;
    case 'objectives':
      return `/w/${slug}/pm/objectives/${id}`;
    case 'documents':
      return `/w/${slug}/docs/documents/${id}`;
    case 'support-conversations':
      return `/w/${slug}/support/${id}`;
    case 'deals':
      return `/w/${slug}/crm/deals/${id}`;
    case 'contacts':
      return `/w/${slug}/crm/contacts/${id}`;
    case 'companies':
      return `/w/${slug}/crm/companies/${id}`;
    case 'agent-runs':
      return `/w/${slug}/pm/coding-sessions/${id}`;
    case 'artifacts':
      return null;
  }
}

/** Remark plugin that preserves Helpin links through the markdown sanitizer. */
export function remarkHelpinReferences() {
  return (tree: unknown) => rewriteHelpinLinkNodes(tree);
}

function decodedReference(type: HelpinReferenceType, encodedID: string): HelpinReference | null {
  try {
    const id = decodeURIComponent(encodedID).trim();
    if (!id || id.includes('/') || id.includes('\\')) return null;
    return { type, id };
  } catch {
    return null;
  }
}

function rewriteHelpinLinkNodes(node: unknown): void {
  if (!node || typeof node !== 'object') return;
  const record = node as { type?: unknown; url?: unknown; children?: unknown };
  if (record.type === 'link' && typeof record.url === 'string') {
    const marker = helpinReferenceMarker(record.url);
    if (marker) record.url = marker;
  }
  if (Array.isArray(record.children)) {
    for (const child of record.children) rewriteHelpinLinkNodes(child);
  }
}
