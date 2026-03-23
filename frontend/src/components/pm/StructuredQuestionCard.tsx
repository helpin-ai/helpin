import { useState } from 'react';
import { Check, Send } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import type { StructuredQuestion } from '@/lib/pmTypes';

export interface StructuredQuestionAnswer {
  value: string;
  label: string;
  freetextValue?: string;
}

function findSelectedOption(question: StructuredQuestion, answer?: StructuredQuestionAnswer | null) {
  if (!answer) return null;
  return question.options.find((option) => option.value === answer.value) ?? null;
}

export function hasAllStructuredQuestionAnswers(
  questions: StructuredQuestion[],
  answers: Record<string, StructuredQuestionAnswer>,
) {
  return questions.every((question) => {
    const answer = answers[question.id];
    if (!answer) return false;
    const selectedOption = findSelectedOption(question, answer);
    if (selectedOption?.freetext) {
      return Boolean(answer.freetextValue?.trim());
    }
    return true;
  });
}

export function formatStructuredQuestionAnswers(
  questions: StructuredQuestion[],
  answers: Record<string, StructuredQuestionAnswer>,
) {
  const serializedAnswers = questions.flatMap((question) => {
    const answer = answers[question.id];
    if (!answer) return [];
    const selectedOption = findSelectedOption(question, answer);
    const selectedLabel = selectedOption?.label ?? answer.label;
    const freetextValue = selectedOption?.freetext ? answer.freetextValue?.trim() : '';

    return [{
      question_id: question.id,
      question: question.text,
      selected_value: answer.value,
      selected_label: selectedLabel,
      freetext: freetextValue || undefined,
    }];
  });

  if (serializedAnswers.length === 0) return '';

  const summaryLines = [
    'Interactive question responses:',
    ...serializedAnswers.map((answer) => (
      answer.freetext
        ? `- ${answer.question_id}: ${answer.question} -> ${answer.selected_label} (${answer.freetext})`
        : `- ${answer.question_id}: ${answer.question} -> ${answer.selected_label}`
    )),
  ];

  return [
    ...summaryLines,
    '',
    '```json',
    JSON.stringify({ answers: serializedAnswers }, null, 2),
    '```',
  ].join('\n');
}

interface Props {
  questions: StructuredQuestion[];
  onSubmit: (formattedAnswer: string) => void;
  disabled?: boolean;
  readOnly?: boolean;
  /** Pre-filled answers for read-only mode (keyed by question id) */
  answers?: Record<string, StructuredQuestionAnswer>;
}

export function StructuredQuestionCard({ questions, onSubmit, disabled, readOnly, answers: initialAnswers }: Props) {
  const [answers, setAnswers] = useState<Record<string, StructuredQuestionAnswer>>(initialAnswers ?? {});
  const allAnswered = hasAllStructuredQuestionAnswers(questions, answers);

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
    const formattedAnswer = formatStructuredQuestionAnswers(questions, answers);
    if (!formattedAnswer) return;
    onSubmit(formattedAnswer);
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
