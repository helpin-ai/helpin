export type DocsBlockKind =
  | 'aiSection'
  | 'citationBlock'
  | 'entityEmbed'
  | 'savedViewEmbed'
  | 'toggleSection'
  | 'fileAttachment'
  | 'tableOfContents'
  | 'richEmbed'
  | 'callout'
  | 'videoEmbed'
  | 'excalidraw'
  | 'resizableImage';

export interface DocsBlockDefinition<Attrs extends Record<string, unknown> = Record<string, unknown>> {
  kind: DocsBlockKind;
  label: string;
  description: string;
  attrs: readonly (keyof Attrs & string)[];
  agentReadableKind: string;
}

export const docsBlockRegistry = {
  aiSection: {
    kind: 'aiSection',
    label: 'Legacy AI Section',
    description: 'Legacy generated section',
    attrs: ['status', 'ownerAgentId', 'lastGeneratedAt', 'sourceCount'],
    agentReadableKind: 'section',
  },
  citationBlock: {
    kind: 'citationBlock',
    label: 'Sources',
    description: 'Citations and provenance',
    attrs: ['title', 'sourceType', 'sourceId', 'confidence'],
    agentReadableKind: 'citation',
  },
  entityEmbed: {
    kind: 'entityEmbed',
    label: 'Entity',
    description: 'Embed PM, CRM, or support record',
    attrs: ['entityType', 'entityId', 'title', 'status'],
    agentReadableKind: 'entity_embed',
  },
  savedViewEmbed: {
    kind: 'savedViewEmbed',
    label: 'Saved View',
    description: 'Embed a live PM, CRM, or support view',
    attrs: ['module', 'viewId', 'viewName'],
    agentReadableKind: 'saved_view_embed',
  },
  toggleSection: {
    kind: 'toggleSection',
    label: 'Toggle',
    description: 'Collapsible section',
    attrs: ['title', 'open'],
    agentReadableKind: 'toggle_section',
  },
  fileAttachment: {
    kind: 'fileAttachment',
    label: 'File',
    description: 'Downloadable attachment',
    attrs: ['fileName', 'fileSize', 'contentType', 'url', 'attachmentId'],
    agentReadableKind: 'file_attachment',
  },
  tableOfContents: {
    kind: 'tableOfContents',
    label: 'Table of Contents',
    description: 'Auto-generated heading links',
    attrs: [],
    agentReadableKind: 'table_of_contents',
  },
  richEmbed: {
    kind: 'richEmbed',
    label: 'Embed',
    description: 'Rich URL unfurl',
    attrs: ['url', 'provider', 'title', 'description'],
    agentReadableKind: 'rich_embed',
  },
  callout: {
    kind: 'callout',
    label: 'Callout',
    description: 'Highlighted note or warning',
    attrs: ['variant'],
    agentReadableKind: 'callout',
  },
  videoEmbed: {
    kind: 'videoEmbed',
    label: 'Video',
    description: 'Embed from YouTube, Vimeo, Loom, Wistia',
    attrs: ['url', 'provider', 'title'],
    agentReadableKind: 'video',
  },
  excalidraw: {
    kind: 'excalidraw',
    label: 'Excalidraw',
    description: 'Editable whiteboard diagram',
    attrs: ['title', 'scene'],
    agentReadableKind: 'diagram',
  },
  resizableImage: {
    kind: 'resizableImage',
    label: 'Image',
    description: 'Upload an image',
    attrs: ['src', 'alt', 'caption', 'align'],
    agentReadableKind: 'image',
  },
} satisfies Record<DocsBlockKind, DocsBlockDefinition>;

export function getDocsBlockDefinition(kind: DocsBlockKind): DocsBlockDefinition {
  return docsBlockRegistry[kind];
}
