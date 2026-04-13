import type { HelpCollection } from './helpApi';

function parseHelpCollectionKey(key: string): { slug: string; publicId: string } | null {
  const trimmed = key.trim().replace(/^\/+|\/+$/g, '');
  const lastDash = trimmed.lastIndexOf('-');
  if (lastDash <= 0 || lastDash === trimmed.length - 1) return null;
  const publicId = trimmed.slice(lastDash + 1).trim().toLowerCase();
  if (!/^[0-9a-f]{8}$/.test(publicId)) return null;
  const slug = trimmed.slice(0, lastDash).trim();
  return slug ? { slug, publicId } : null;
}

/**
 * A folded widget collection tree node: one collection plus its
 * direct children. Articles are fetched per-collection by the
 * HelpCollectionView component and are not stored on the tree.
 */
export interface HelpCollectionTreeNode {
  collection: HelpCollection;
  children: HelpCollectionTreeNode[];
}

/**
 * buildHelpCollectionTree folds a flat HelpCollection array (as returned
 * by the widget API after Task 6) into a nested tree keyed on
 * parent_collection_id. Orphans whose parent is missing from the response
 * are promoted to the top level so a dirty snapshot cannot hide content.
 */
export function buildHelpCollectionTree(collections: HelpCollection[]): HelpCollectionTreeNode[] {
  const byId = new Map<string, HelpCollectionTreeNode>();
  for (const c of collections) {
    byId.set(c.id, { collection: c, children: [] });
  }
  const roots: HelpCollectionTreeNode[] = [];
  for (const c of collections) {
    const node = byId.get(c.id)!;
    if (c.parent_collection_id && byId.has(c.parent_collection_id)) {
      byId.get(c.parent_collection_id)!.children.push(node);
    } else {
      roots.push(node);
    }
  }
  return roots;
}

/**
 * findHelpCollectionBySlug walks the tree depth-first and returns the
 * first node whose collection.slug matches. Used to locate the active
 * collection on drilldown before rendering its children.
 */
export function findHelpCollectionBySlug(
  tree: HelpCollectionTreeNode[],
  slug: string,
): HelpCollectionTreeNode | null {
  const parsed = parseHelpCollectionKey(slug);
  for (const node of tree) {
    if (parsed ? node.collection.public_id === parsed.publicId : node.collection.slug === slug) return node;
    const hit = findHelpCollectionBySlug(node.children, slug);
    if (hit) return hit;
  }
  return null;
}

/**
 * helpCollectionAncestorPath returns the ancestor chain of a collection
 * ordered top-down (root first, target collection last). Used by the
 * widget to render a compact breadcrumb trail at the top of a drilldown
 * collection view.
 */
export function helpCollectionAncestorPath(
  tree: HelpCollectionTreeNode[],
  slug: string,
): HelpCollectionTreeNode[] {
  const parsed = parseHelpCollectionKey(slug);
  const path: HelpCollectionTreeNode[] = [];
  const walk = (nodes: HelpCollectionTreeNode[]): boolean => {
    for (const node of nodes) {
      path.push(node);
      if (parsed ? node.collection.public_id === parsed.publicId : node.collection.slug === slug) return true;
      if (walk(node.children)) return true;
      path.pop();
    }
    return false;
  };
  walk(tree);
  return path;
}
