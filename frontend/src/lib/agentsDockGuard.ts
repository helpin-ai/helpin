/**
 * The Ask Agents dock is portal'd to document.body and floats above sheets/dialogs.
 * Radix dismissable layers (Sheet, Dialog) treat clicks inside the dock as "outside"
 * clicks and close themselves. Use this guard inside `onPointerDownOutside` /
 * `onInteractOutside` to ignore those clicks.
 *
 * Pair with the `data-helpin-dock="true"` attribute on the dock root.
 */
export function isInsideAskAgentsDock(target: EventTarget | null | undefined): boolean {
  if (!(target instanceof Element)) return false;
  return target.closest('[data-helpin-dock]') !== null;
}
