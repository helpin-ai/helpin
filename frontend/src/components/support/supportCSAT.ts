const RATING_LABELS = ['Very unsatisfied', 'Unsatisfied', 'Neutral', 'Satisfied', 'Very satisfied']

export interface SupportCSATEventDetails {
  rating: number
  comment?: string
  previousRating?: number
}

/** Reads the rating stored on a `csat_rated` timeline event. */
export function parseSupportCSATEvent(metadata?: string): SupportCSATEventDetails | null {
  if (!metadata?.trim()) return null
  try {
    const parsed = JSON.parse(metadata) as { rating?: unknown; comment?: unknown; previous_rating?: unknown }
    const rating = typeof parsed.rating === 'number' ? parsed.rating : NaN
    if (!Number.isInteger(rating) || rating < 1 || rating > 5) return null
    return {
      rating,
      comment: typeof parsed.comment === 'string' && parsed.comment.trim() ? parsed.comment.trim() : undefined,
      previousRating: typeof parsed.previous_rating === 'number' ? parsed.previous_rating : undefined,
    }
  } catch {
    return null
  }
}

export function supportCSATRatingLabel(rating: number): string {
  return RATING_LABELS[rating - 1] ?? ''
}
