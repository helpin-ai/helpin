import { h, FunctionComponent } from 'preact';
import { useState } from 'preact/hooks';

interface CsatRatingProps {
  onSubmit: (rating: number, feedback?: string) => void;
}

const EMOJI_RATINGS = [
  { value: 1, emoji: '😞' },
  { value: 2, emoji: '😕' },
  { value: 3, emoji: '😐' },
  { value: 4, emoji: '🙂' },
  { value: 5, emoji: '😄' },
];

export const CsatRating: FunctionComponent<CsatRatingProps> = ({ onSubmit }) => {
  const [rating, setRating] = useState<number | null>(null);
  const [feedback, setFeedback] = useState('');
  const [submitted, setSubmitted] = useState(false);

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

  return (
    <div className="helpin-csat-rating">
      <div className="helpin-csat-question">How would you rate your experience?</div>
      <div className="helpin-csat-emojis">
        {EMOJI_RATINGS.map(({ value, emoji }) => (
          <button
            key={value}
            className={`helpin-csat-emoji ${rating === value ? 'helpin-csat-emoji--selected' : ''}`}
            onClick={() => setRating(value)}
          >
            {emoji}
          </button>
        ))}
      </div>
      {rating !== null && (
        <div className="helpin-csat-feedback">
          <textarea
            placeholder="Any additional feedback?"
            value={feedback}
            onInput={(e) => setFeedback((e.target as HTMLTextAreaElement).value)}
          />
          <button onClick={handleSubmit} className="helpin-btn-primary">
            Submit
          </button>
        </div>
      )}
    </div>
  );
};
