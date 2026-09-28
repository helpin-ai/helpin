import { format, isValid, parseISO } from 'date-fns';

type Health = { label: string | null; tone: 'neutral' | 'positive' | 'warning' | 'negative'; explanation: string };
const neutral = (explanation: string): Health => ({ label: null, tone: 'neutral', explanation });
const calendarDay = (value?: string) => {
  if (!value) return null;
  const date = parseISO(value.slice(0, 10));
  return isValid(date) ? Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()) : null;
};

export function keyResultHealth(progress: number, startDate?: string, deadline?: string, now = new Date(), isCompletion = false): Health {
  if (progress >= 100) return { label: 'Complete', tone: 'positive', explanation: 'Target reached.' };
  const start = calendarDay(startDate), end = calendarDay(deadline);
  const today = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  if (start !== null && end !== null && end < start) return neutral('The target date is before the start date.');
  if (end !== null && today > end) return { label: 'Overdue', tone: 'negative', explanation: `Target was ${format(parseISO(deadline!.slice(0, 10)), 'MMM d, yyyy')}.` };
  if (start === null || end === null) return neutral('Set objective start and target dates to assess progress against time.');
  if (today < start) return neutral('The objective has not started yet.');
  if (isCompletion) return neutral('Completion is tracked as done or not done, rather than a steady pace.');
  const expected = end === start ? 100 : Math.min(100, (today - start) / (end - start) * 100);
  const gap = expected - progress;
  return {
    label: gap <= 0 ? 'On track' : gap <= 10 ? 'Slightly behind' : 'Behind',
    tone: gap <= 0 ? 'positive' : gap <= 10 ? 'warning' : 'negative',
    explanation: `Expected ${Math.round(expected)}% by today, assuming steady progress between the objective’s start and target dates.`,
  };
}
