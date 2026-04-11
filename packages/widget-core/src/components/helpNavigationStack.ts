/**
 * helpNavigationStack — pure state helpers for the widget's help view
 * back-button behaviour. Extracted so the drilldown semantics can be
 * unit-tested without spinning up the full ChatWindow + fetch stack.
 */

export type HelpCollectionBackTarget =
  | { kind: 'collection'; slug: string; remainingStack: string[] }
  | { kind: 'help-space' }
  | { kind: 'help' };

/**
 * Computes the next view target when the user presses "back" from a
 * help-collection view.
 *
 * Rules (matching ChatWindow.handleBackFromHelpCollection):
 *   1. If the breadcrumb stack has entries, pop the nearest ancestor
 *      and return it as the new active collection. The caller should
 *      also store the shortened stack.
 *   2. Otherwise, if the widget has multiple help spaces and an
 *      active space is set, return to the help-space list.
 *   3. Otherwise fall through to the top-level help view.
 */
export function computeHelpCollectionBackTarget(
  stack: string[],
  helpSpaceCount: number,
  activeHelpSpaceSlug: string | null,
): HelpCollectionBackTarget {
  if (stack.length > 0) {
    const slug = stack[stack.length - 1]!;
    return {
      kind: 'collection',
      slug,
      remainingStack: stack.slice(0, -1),
    };
  }
  if (helpSpaceCount > 1 && activeHelpSpaceSlug) {
    return { kind: 'help-space' };
  }
  return { kind: 'help' };
}

/**
 * Computes the next breadcrumb stack when the user drills into a child
 * collection from within a help-collection view.
 *
 * Rules:
 *   - When the user is already on a collection page, push the current
 *     slug onto the stack so it becomes the immediate ancestor of the
 *     new child.
 *   - When drilling in from home or the help-space list, reset the
 *     stack so subsequent back presses don't surface unrelated
 *     ancestors.
 *   - Re-opening the same collection slug is a no-op on the stack.
 */
export function pushHelpCollectionOnDrilldown(
  currentStack: string[],
  activeView: string,
  activeCollectionSlug: string | null,
  nextCollectionSlug: string,
): string[] {
  if (
    activeView === 'help-collection' &&
    activeCollectionSlug &&
    activeCollectionSlug !== nextCollectionSlug
  ) {
    return [...currentStack, activeCollectionSlug];
  }
  return [];
}
