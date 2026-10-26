import { StarIcon } from '@/lib/icons'
import { cn } from '@/lib/utils'
import { supportCSATRatingLabel, type SupportCSATEventDetails } from './supportCSAT'

export function SupportCSATStars({ rating, className }: { rating: number; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-0.5', className)} role="img" aria-label={`${rating} out of 5`}>
      {[1, 2, 3, 4, 5].map((value) => (
        <StarIcon
          key={value}
          className={cn('h-3.5 w-3.5', value <= rating ? 'fill-amber-400 text-amber-500' : 'text-muted-foreground/40')}
        />
      ))}
    </span>
  )
}

/** Timeline card for a customer's satisfaction rating. */
export function SupportCSATEvent({
  details,
  customerName,
  timestamp,
}: {
  details: SupportCSATEventDetails
  customerName: string
  timestamp: string
}) {
  const changed = details.previousRating !== undefined && details.previousRating !== details.rating
  return (
    <div className="my-5 flex justify-center animate-in fade-in duration-300" data-support-csat-event>
      <div className="max-w-[85%] rounded-xl border border-amber-200/80 bg-amber-50/60 px-4 py-2.5 text-xs shadow-sm dark:border-amber-900/50 dark:bg-amber-950/20">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-amber-950 dark:text-amber-100">
          <span className="font-medium">{customerName || 'Customer'} {changed ? 'changed their rating' : 'rated this conversation'}</span>
          <SupportCSATStars rating={details.rating} />
          <span className="text-amber-900/80 dark:text-amber-200/80">{supportCSATRatingLabel(details.rating)}</span>
          {changed ? <span className="text-amber-900/60 dark:text-amber-200/60">(was {details.previousRating}/5)</span> : null}
          <span className="ml-auto text-amber-900/50 dark:text-amber-200/50">{timestamp}</span>
        </div>
        {details.comment ? (
          <p className="mt-1.5 whitespace-pre-wrap text-[13px] leading-relaxed text-foreground/85 [overflow-wrap:anywhere]">“{details.comment}”</p>
        ) : null}
      </div>
    </div>
  )
}
