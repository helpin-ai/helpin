import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

const RATINGS = [
  { value: 1, emoji: '😞', label: 'Very unsatisfied' },
  { value: 2, emoji: '😕', label: 'Unsatisfied' },
  { value: 3, emoji: '😐', label: 'Neutral' },
  { value: 4, emoji: '🙂', label: 'Satisfied' },
  { value: 5, emoji: '😄', label: 'Very satisfied' },
]

interface PortalRatingCardProps {
  current?: { rating: number; comment?: string | null } | null
  saving: boolean
  error?: string | null
  onSubmit: (rating: number, comment: string) => Promise<boolean>
}

/** Asks the customer to rate a resolved request, and lets them change it. */
export function PortalRatingCard({ current, saving, error, onSubmit }: PortalRatingCardProps) {
  const [editing, setEditing] = useState(!current)
  const [rating, setRating] = useState<number | null>(current?.rating ?? null)
  const [comment, setComment] = useState(current?.comment ?? '')

  if (current && !editing) {
    const label = RATINGS.find((option) => option.value === current.rating)
    return (
      <section className="mt-6 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-quiet-divider-strong pt-4 text-sm" aria-label="Your rating">
        <span className="text-quiet-text-secondary">
          Thanks for your feedback. You rated this request {current.rating}/5{label ? ` (${label.label.toLowerCase()})` : ''}.
        </span>
        <Button variant="link" size="sm" className="h-auto p-0" onClick={() => setEditing(true)}>Change rating</Button>
      </section>
    )
  }

  return (
    <section className="mt-6 border-t border-quiet-divider-strong pt-4" aria-labelledby="portal-rating-title">
      <h2 id="portal-rating-title" className="text-sm font-semibold text-quiet-text-primary">How was the support you received?</h2>
      <p className="mt-1 text-[13px] text-quiet-text-tertiary">Your rating goes to the team that handled this request.</p>
      <div className="mt-3 flex flex-wrap gap-2" role="radiogroup" aria-label="Rate this request">
        {RATINGS.map((option) => (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={rating === option.value}
            aria-label={option.label}
            title={option.label}
            onClick={() => setRating(option.value)}
            className={cn(
              'flex h-11 w-11 items-center justify-center rounded-full border text-xl transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-quiet-text-primary',
              rating === option.value ? 'border-quiet-text-primary bg-quiet-row-hover' : 'border-quiet-divider-strong hover:bg-quiet-row-hover',
            )}
          >
            <span aria-hidden="true">{option.emoji}</span>
          </button>
        ))}
      </div>
      {rating !== null ? (
        <form
          className="mt-3 space-y-2"
          onSubmit={async (event) => {
            event.preventDefault()
            if (await onSubmit(rating, comment)) setEditing(false)
          }}
        >
          <label htmlFor="portal-rating-comment" className="sr-only">Comment</label>
          <Textarea
            id="portal-rating-comment"
            value={comment}
            maxLength={1000}
            onChange={(event) => setComment(event.target.value)}
            placeholder={rating <= 3 ? 'What could we have done better? (optional)' : 'What worked well? (optional)'}
            className="min-h-20"
          />
          {error ? <p role="alert" className="text-[13px] text-red-600 dark:text-red-400">{error}</p> : null}
          <div className="flex items-center gap-2">
            <Button type="submit" size="sm" disabled={saving}>{saving ? 'Sending…' : 'Send rating'}</Button>
            {current ? <Button type="button" variant="ghost" size="sm" onClick={() => setEditing(false)}>Cancel</Button> : null}
          </div>
        </form>
      ) : null}
    </section>
  )
}
