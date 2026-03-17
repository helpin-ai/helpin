import { useState } from 'react';
import { Check, Send } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import type { StructuredQuestion } from '@/lib/pmTypes';

interface Answer {
  value: string;
  label: string;
  freetextValue?: string;
}

interface Props {
  questions: StructuredQuestion[];
  onSubmit: (formattedAnswer: string) => void;
  disabled?: boolean;
  readOnly?: boolean;
  /** Pre-filled answers for read-only mode (keyed by question id) */
  answers?: Record<string, Answer>;
}

export function StructuredQuestionCard({ questions, onSubmit, disabled, readOnly, answers: initialAnswers }: Props) {
  const [answers, setAnswers] = useState<Record<string, Answer>>(initialAnswers ?? {});

  const allAnswered = questions.every((q) => {
    const a = answers[q.id];
    if (!a) return false;
    if (a.value === 'other' && !a.freetextValue?.trim()) return false;
    return true;
  });

  const handleSelect = (questionId: string, value: string, label: string, freetext?: boolean) => {
    setAnswers((prev) => ({
      ...prev,
      [questionId]: { value, label, freetextValue: freetext ? prev[questionId]?.freetextValue : undefined },
    }));
  };

  const handleFreetext = (questionId: string, text: string) => {
    setAnswers((prev) => ({
      ...prev,
      [questionId]: { ...prev[questionId], freetextValue: text },
    }));
  };

  const handleSubmit = () => {
    const lines = questions.map((q) => {
      const a = answers[q.id];
      if (!a) return '';
      const answerText = a.value === 'other' && a.freetextValue?.trim()
        ? `other: ${a.freetextValue.trim()}`
        : `${a.value}: ${a.label}`;
      return `${q.id.toUpperCase()}: ${q.text}\n→ ${answerText}`;
    });
    onSubmit(lines.filter(Boolean).join('\n\n'));
  };

  return (
    <div className={cn(
      'mt-2 rounded-lg border-2 p-3 space-y-4',
      readOnly
        ? 'border-border/40 bg-muted/20'
        : 'border-indigo-300 bg-indigo-50/30 dark:border-indigo-700 dark:bg-indigo-950/20',
    )}>
      {questions.map((q) => {
        const selected = answers[q.id];
        return (
          <div key={q.id} className="space-y-2">
            <p className="text-xs font-medium">{q.text}</p>
            <div className="space-y-1">
              {q.options.map((opt) => {
                const isSelected = selected?.value === opt.value;
                return (
                  <div key={opt.value}>
                    <label
                      className={cn(
                        'flex cursor-pointer items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs transition-colors',
                        readOnly && !isSelected && 'hidden',
                        isSelected
                          ? 'border-indigo-400 bg-indigo-100/60 dark:border-indigo-600 dark:bg-indigo-900/30'
                          : 'border-border/50 hover:bg-muted/40',
                        (readOnly || disabled) && 'pointer-events-none',
                      )}
                    >
                      <input
                        type="radio"
                        name={q.id}
                        value={opt.value}
                        checked={isSelected}
                        onChange={() => handleSelect(q.id, opt.value, opt.label, opt.freetext)}
                        disabled={readOnly || disabled}
                        className="sr-only"
                      />
                      <span className={cn(
                        'flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border',
                        isSelected
                          ? 'border-indigo-500 bg-indigo-500'
                          : 'border-muted-foreground/40',
                      )}>
                        {isSelected && <Check className="h-2 w-2 text-white" />}
                      </span>
                      <span>{opt.label}</span>
                    </label>
                    {opt.freetext && isSelected && (
                      <Input
                        value={selected?.freetextValue ?? ''}
                        onChange={(e) => handleFreetext(q.id, e.target.value)}
                        placeholder="Please specify..."
                        className="mt-1 ml-6 h-7 text-xs"
                        disabled={readOnly || disabled}
                        autoFocus
                      />
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}

      {!readOnly && (
        <Button
          size="sm"
          onClick={handleSubmit}
          disabled={!allAnswered || disabled}
          className="w-full"
        >
          <Send className="mr-1.5 h-3 w-3" />
          Submit Answers
        </Button>
      )}
    </div>
  );
}
