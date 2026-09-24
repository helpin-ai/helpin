/** Keep DOM and React previews on the same soft streaming cadence. */
export function streamCharacterDelay(index: number, length: number, duration: number, offset = 0) {
  return offset + index * Math.max(0, duration - 160) / Math.max(1, length - 1);
}
