import { FunctionComponent } from 'preact';
import { useState } from 'preact/hooks';

interface CsatRatingProps {
  onSubmit: (rating: number, feedback?: string) => void;
}

const EMOJI_RATINGS = [
  { value: 1, emoji: '😞', label: 'Very unsatisfied' },
  { value: 2, emoji: '😕', label: 'Unsatisfied' },
  { value: 3, emoji: '😐', label: 'Neutral' },
  { value: 4, emoji: '🙂', label: 'Satisfied' },
  { value: 5, emoji: '😄', label: 'Very satisfied' },
];

export const CsatRating: FunctionComponent<CsatRatingProps> = ({ onSubmit }) => {
  const [rating, setRating] = useState<number | null>(null);
  const [feedback, setFeedback] = useState('');
  const [submitted, setSubmitted] = useState(false);
  const [dismissed, setDismissed] = useState(false);

  const handleSubmit = () => {
    if (rating !== null) {
      onSubmit(rating, feedback);
      setSubmitted(true);
    }
  };

  if (submitted) {
    return (
      <div className="helpin-csat-rating helpin-csat-rating--submitted">
        Thank you for your feedback!
      </div>
    );
  }

  if (dismissed) return null;

  return (
    <div className="helpin-csat-rating">
      <div className="helpin-csat-eyebrow">Conversation resolved</div>
      <div className="helpin-csat-question">How was your support experience?</div>
      <div className="helpin-csat-description">Your rating helps the team improve future answers.</div>
      <div className="helpin-csat-emojis" role="radiogroup" aria-label="Rate your experience">
        {EMOJI_RATINGS.map(({ value, emoji, label }) => (
          <button
            key={value}
            type="button"
            className={`helpin-csat-emoji ${rating === value ? 'helpin-csat-emoji--selected' : ''}`}
            onClick={() => setRating(value)}
            role="radio"
            aria-checked={rating === value}
            aria-label={label}
          >
            {emoji}
          </button>
        ))}
      </div>
      {rating !== null && (
        <div className="helpin-csat-feedback">
          <textarea
            placeholder={rating <= 3 ? 'What could we improve? (optional)' : 'What worked well? (optional)'}
            value={feedback}
            onInput={(e) => setFeedback((e.target as HTMLTextAreaElement).value)}
            aria-label="Additional feedback"
            maxLength={1000}
          />
          <button type="button" onClick={handleSubmit} className="helpin-csat-submit">
            Send feedback
          </button>
        </div>
      )}
      {rating === null && (
        <button type="button" className="helpin-csat-dismiss" onClick={() => setDismissed(true)}>
          Not now
        </button>
      )}
    </div>
  );
};
