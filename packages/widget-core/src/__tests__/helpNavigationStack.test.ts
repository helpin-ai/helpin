import { describe, it, expect } from 'vitest';
import {
  computeHelpCollectionBackTarget,
  pushHelpCollectionOnDrilldown,
} from '../components/helpNavigationStack';

describe('pushHelpCollectionOnDrilldown', () => {
  it('pushes the current slug when drilling from a collection view', () => {
    const next = pushHelpCollectionOnDrilldown(
      [],
      'help-collection',
      'parent',
      'child',
    );
    expect(next).toEqual(['parent']);
  });

  it('appends to an existing stack on a deeper drilldown', () => {
    const next = pushHelpCollectionOnDrilldown(
      ['root'],
      'help-collection',
      'mid',
      'leaf',
    );
    expect(next).toEqual(['root', 'mid']);
  });

  it('resets the stack when drilling from a non-collection view', () => {
    const next = pushHelpCollectionOnDrilldown(
      ['stale'],
      'help-space',
      null,
      'top-level',
    );
    expect(next).toEqual([]);
  });

  it('does not push when opening the same slug the user is already on', () => {
    const next = pushHelpCollectionOnDrilldown(
      ['root'],
      'help-collection',
      'leaf',
      'leaf',
    );
    // Same-slug open behaves like a fresh nav — no change to the stack.
    expect(next).toEqual([]);
  });
});

describe('computeHelpCollectionBackTarget', () => {
  it('pops the nearest ancestor when the stack has entries', () => {
    const target = computeHelpCollectionBackTarget(['root', 'mid'], 1, 'space');
    expect(target).toEqual({
      kind: 'collection',
      slug: 'mid',
      remainingStack: ['root'],
    });
  });

  it('walks to an empty stack after enough pops', () => {
    const first = computeHelpCollectionBackTarget(['root'], 1, 'space');
    expect(first.kind).toBe('collection');
    if (first.kind !== 'collection') throw new Error('expected collection target');
    const second = computeHelpCollectionBackTarget(first.remainingStack, 1, 'space');
    // Only one space configured -> fall through to home.
    expect(second.kind).toBe('help');
  });

  it('returns help-space when multiple spaces are configured and stack is empty', () => {
    const target = computeHelpCollectionBackTarget([], 3, 'engineering');
    expect(target).toEqual({ kind: 'help-space' });
  });

  it('returns help when no active space is set', () => {
    const target = computeHelpCollectionBackTarget([], 2, null);
    expect(target).toEqual({ kind: 'help' });
  });

  it('returns help when only one space is configured', () => {
    const target = computeHelpCollectionBackTarget([], 1, 'only');
    expect(target).toEqual({ kind: 'help' });
  });
});

describe('combined drilldown + back navigation flow', () => {
  it('walks root -> mid -> leaf and backs out one level at a time', () => {
    // Start at root from a fresh nav.
    let stack = pushHelpCollectionOnDrilldown([], 'help-space', null, 'root');
    expect(stack).toEqual([]);

    // Drill into mid.
    stack = pushHelpCollectionOnDrilldown(stack, 'help-collection', 'root', 'mid');
    expect(stack).toEqual(['root']);

    // Drill into leaf.
    stack = pushHelpCollectionOnDrilldown(stack, 'help-collection', 'mid', 'leaf');
    expect(stack).toEqual(['root', 'mid']);

    // Back from leaf -> lands on mid with stack=[root].
    const backToMid = computeHelpCollectionBackTarget(stack, 1, 'space');
    if (backToMid.kind !== 'collection') throw new Error('expected collection');
    expect(backToMid.slug).toBe('mid');
    stack = backToMid.remainingStack;
    expect(stack).toEqual(['root']);

    // Back from mid -> lands on root with stack=[].
    const backToRoot = computeHelpCollectionBackTarget(stack, 1, 'space');
    if (backToRoot.kind !== 'collection') throw new Error('expected collection');
    expect(backToRoot.slug).toBe('root');
    stack = backToRoot.remainingStack;
    expect(stack).toEqual([]);

    // Back from root with only one space -> home.
    const backToHome = computeHelpCollectionBackTarget(stack, 1, 'space');
    expect(backToHome.kind).toBe('help');
  });
});
