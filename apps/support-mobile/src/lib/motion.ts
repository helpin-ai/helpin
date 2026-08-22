// Shared `motion/react` presets so animated primitives across the app move
// with the same feel instead of each component picking its own numbers.

/** Springy layout transition (e.g. SegmentedControl's sliding thumb). */
export const stackSpring = { type: 'spring', stiffness: 380, damping: 38 } as const

/** Quick, snappy feedback for press/tap states. */
export const pressTransition = { duration: 0.1 } as const

/** Gentle rise-and-fade entrance for cards, list items, and sheets. */
export const riseIn = {
  initial: { opacity: 0, y: 12 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.22, ease: 'easeOut' },
} as const
