import { describe, it, expect } from 'vitest';
import {
  buildHelpCollectionTree,
  findHelpCollectionBySlug,
  helpCollectionAncestorPath,
} from '../components/helpTree';
import type { HelpCollection } from '../components/helpApi';

function makeCollection(
  id: string,
  parent: string | null,
  depth: number,
  slug = id,
  articleCount = 0,
): HelpCollection {
  return {
    id,
    name: id,
    slug,
    parent_collection_id: parent,
    depth,
    article_count: articleCount,
  };
}

describe('buildHelpCollectionTree', () => {
  it('folds a flat list into a nested tree', () => {
    const collections = [
      makeCollection('root', null, 0),
      makeCollection('a', 'root', 1),
      makeCollection('b', 'root', 1),
      makeCollection('a1', 'a', 2),
    ];
    const tree = buildHelpCollectionTree(collections);
    expect(tree).toHaveLength(1);
    const root = tree[0]!;
    expect(root.collection.id).toBe('root');
    expect(root.children.map((n) => n.collection.id)).toEqual(['a', 'b']);
    expect(root.children[0]!.children.map((n) => n.collection.id)).toEqual(['a1']);
  });

  it('promotes orphaned nodes to top-level', () => {
    const tree = buildHelpCollectionTree([makeCollection('orphan', 'missing', 1)]);
    expect(tree).toHaveLength(1);
    expect(tree[0]!.collection.id).toBe('orphan');
  });
});

describe('findHelpCollectionBySlug', () => {
  const collections = [
    makeCollection('root', null, 0, 'root-slug'),
    makeCollection('mid', 'root', 1, 'mid-slug'),
    makeCollection('leaf', 'mid', 2, 'leaf-slug'),
  ];
  const tree = buildHelpCollectionTree(collections);

  it('finds a top-level collection', () => {
    expect(findHelpCollectionBySlug(tree, 'root-slug')?.collection.id).toBe('root');
  });

  it('finds a nested collection', () => {
    expect(findHelpCollectionBySlug(tree, 'leaf-slug')?.collection.id).toBe('leaf');
  });

  it('returns null for an unknown slug', () => {
    expect(findHelpCollectionBySlug(tree, 'ghost')).toBeNull();
  });
});

describe('helpCollectionAncestorPath', () => {
  const collections = [
    makeCollection('root', null, 0, 'root-slug'),
    makeCollection('mid', 'root', 1, 'mid-slug'),
    makeCollection('leaf', 'mid', 2, 'leaf-slug'),
  ];
  const tree = buildHelpCollectionTree(collections);

  it('returns the full path top-down for a leaf', () => {
    const path = helpCollectionAncestorPath(tree, 'leaf-slug');
    expect(path.map((n) => n.collection.id)).toEqual(['root', 'mid', 'leaf']);
  });

  it('returns a single entry for the root', () => {
    const path = helpCollectionAncestorPath(tree, 'root-slug');
    expect(path.map((n) => n.collection.id)).toEqual(['root']);
  });

  it('returns empty for an unknown slug', () => {
    expect(helpCollectionAncestorPath(tree, 'ghost')).toEqual([]);
  });
});
