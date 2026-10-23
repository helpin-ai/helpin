// Picks the words shown when a person clears an inbox: the rarest achievement
// that fits the moment, then wording the viewer has not seen recently.
// Numbers appear only when the number is the achievement.

import type { SupportInboxClearStats } from '@/lib/pmTypes';

export type InboxClearedRarity = 'common' | 'rare' | 'epic' | 'legendary';

/** Facts the server returns when it records a clear. */
export type InboxClearedStats = SupportInboxClearStats;

export interface InboxClearedContext {
  /** Inbox name as it starts a sentence, e.g. "Billing" or "Your inbox". */
  inbox: string;
  /** Inbox name inside a sentence, e.g. "Billing" or "your inbox". */
  inboxInSentence: string;
  /** Viewer's local hour, 0–23. */
  hour: number;
  /** Viewer's local weekday, 0 = Sunday. */
  day: number;
  /** Server facts; null when they could not be loaded. */
  stats: InboxClearedStats | null;
}

export interface InboxClearedMessage {
  achievementId: string;
  rarity: InboxClearedRarity;
  name: string;
  title: string;
  body: string;
}

/** What the viewer has already seen, kept per user in the browser. */
export interface InboxClearedMemory {
  /** Recent names, titles and lines, oldest first. */
  recent: string[];
  /** "<scopeKey>:<achievementId>" → local date it last unlocked. */
  unlocked: Record<string, string>;
}

export const INBOX_CLEARED_RECENT_LIMIT = 20;

export const RARITY_LABEL: Record<InboxClearedRarity, string> = {
  common: 'Common',
  rare: 'Rare',
  epic: 'Epic',
  legendary: 'Legendary',
};

const RARITY_ORDER: InboxClearedRarity[] = ['legendary', 'epic', 'rare', 'common'];
const DAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
const ORDINALS = ['zeroth', 'first', 'second', 'third', 'fourth', 'fifth', 'sixth', 'seventh', 'eighth', 'ninth', 'tenth'];

interface Variant {
  title: string;
  body: string;
}

interface Achievement {
  id: string;
  rarity: Exclude<InboxClearedRarity, 'common'>;
  name: string;
  when: (ctx: InboxClearedContext) => boolean;
  variants: Variant[];
}

const stat = (ctx: InboxClearedContext, key: keyof InboxClearedStats) => ctx.stats?.[key] ?? 0;

const ACHIEVEMENTS: Achievement[] = [
  {
    id: 'unbroken', rarity: 'legendary', name: 'Unbroken',
    when: (ctx) => stat(ctx, 'streak_days') >= 30,
    variants: [
      { title: '{streak} days in a row.', body: 'An inbox at zero, every single day.' },
      { title: 'A month of inbox zero.', body: '{streak} days in a row, nobody was left waiting.' },
    ],
  },
  {
    id: 'marathon', rarity: 'legendary', name: 'Marathon',
    when: (ctx) => stat(ctx, 'resolved_today') >= 50,
    variants: [
      { title: '{resolvedToday} resolved today.', body: 'Most teams call that a week. You called it a day.' },
      { title: 'That was a marathon.', body: 'You resolved {resolvedToday} conversations today.' },
    ],
  },
  {
    id: 'hot-streak', rarity: 'epic', name: 'Hot streak',
    when: (ctx) => stat(ctx, 'streak_days') >= 7 && stat(ctx, 'streak_days') < 30,
    variants: [
      { title: 'Day {streak} of inbox zero.', body: 'More than a week of nobody left waiting.' },
      { title: '{streak} days running.', body: 'Every day, you got to zero. Keep it going.' },
    ],
  },
  {
    id: 'mountain', rarity: 'epic', name: 'Mountain moved',
    when: (ctx) => stat(ctx, 'cleared_count') >= 20,
    variants: [
      { title: '{cleared} down. Zero to go.', body: 'That was a lot to get through.' },
      { title: 'That was a mountain.', body: 'You closed {cleared} conversations to get here.' },
    ],
  },
  {
    id: 'encore', rarity: 'rare', name: 'Encore',
    when: (ctx) => stat(ctx, 'clears_today') >= 2,
    variants: [
      { title: 'Zero again.', body: '{Inbox} hit zero for the {ordinalClears} time today.' },
      { title: '{OrdinalClears} clear today.', body: 'It keeps coming. You keep clearing it.' },
    ],
  },
  {
    id: 'early-bird', rarity: 'rare', name: 'Early bird',
    when: (ctx) => ctx.hour >= 4 && ctx.hour < 9,
    variants: [
      { title: 'Clear before 9.', body: 'The rest of the morning is yours.' },
      { title: '{Inbox} is done before most people log in.', body: 'Customers are waking up to answers.' },
    ],
  },
  {
    id: 'night-owl', rarity: 'rare', name: 'Night owl',
    when: (ctx) => ctx.hour >= 21 || ctx.hour < 4,
    variants: [
      { title: 'Last one out turns off the lights.', body: 'Nobody is waiting until morning. Get some rest.' },
      { title: '{Inbox} is clear for the night.', body: 'Everyone who wrote today has an answer.' },
    ],
  },
  {
    id: 'lunch', rarity: 'rare', name: 'Lunch, earned',
    when: (ctx) => ctx.hour >= 11 && ctx.hour < 13,
    variants: [
      { title: 'Clear before lunch.', body: 'Nothing will be waiting when you get back.' },
      { title: 'Lunch break, fully earned.', body: 'Nobody in {inbox} is waiting.' },
    ],
  },
  {
    id: 'friday', rarity: 'rare', name: 'Weekend-ready',
    when: (ctx) => ctx.day === 5 && ctx.hour >= 14 && ctx.hour < 21,
    variants: [
      { title: 'Weekend-ready.', body: 'Nothing to carry into the weekend. Have a good one.' },
      { title: 'Friday, finished.', body: '{Inbox} is clear. The weekend can start.' },
    ],
  },
  {
    id: 'monday', rarity: 'rare', name: 'Monday, handled',
    when: (ctx) => ctx.day === 1 && ctx.hour >= 9 && ctx.hour < 21,
    variants: [
      { title: 'Monday, handled.', body: 'Everything from the weekend has an answer.' },
      { title: 'The weekend pile is gone.', body: 'The week starts at zero.' },
    ],
  },
  {
    id: 'weekend', rarity: 'rare', name: 'Weekend shift',
    when: (ctx) => (ctx.day === 0 || ctx.day === 6) && ctx.hour >= 9 && ctx.hour < 21,
    variants: [
      { title: 'Clear on a {dayName}.', body: 'Customers who wrote this weekend are not waiting for Monday.' },
      { title: 'Weekend shift, done.', body: 'Thanks for covering the weekend.' },
    ],
  },
];

const COMMON_NAMES = ['Clean sweep', 'All caught up', 'Zero hour', 'Last one out', 'Spotless', 'Quiet hour', 'Nothing waiting'];
const COMMON_TITLES = [
  '{Inbox} is clear.',
  'Inbox zero. Nicely done.',
  'Not a single one left.',
  'That was the last one.',
  'All quiet in {inbox}.',
  'Clean sweep.',
  'Every customer has an answer.',
  'You got to the bottom of it.',
  'Nothing waiting.',
  'Zero. Lovely number.',
  'And that is a wrap.',
  'Caught up, for real.',
  'Done and dusted.',
  'Empty, the way it should be.',
];
const COMMON_LINES = [
  'Nobody in {inbox} is waiting on a reply.',
  'Every question that came in has an answer.',
  'Every customer here has heard back.',
  'The queue is empty. Enjoy it.',
  'Take the win.',
  'New conversations will land here.',
  'Nice work.',
];
const TIME_LINES: Record<'morning' | 'midday' | 'afternoon' | 'evening', string[]> = {
  morning: ['The rest of the morning is yours.', 'Coffee tastes better now.'],
  midday: ['Lunch tastes better now.', 'Good moment for a walk.'],
  afternoon: ['Stretch. You earned it.', 'Enjoy the quiet while it lasts.'],
  evening: ['Good place to stop for the day.', 'Time to log off?'],
};

function partOfDay(hour: number): keyof typeof TIME_LINES {
  if (hour >= 5 && hour < 11) return 'morning';
  if (hour >= 11 && hour < 14) return 'midday';
  if (hour >= 14 && hour < 18) return 'afternoon';
  return 'evening';
}

function ordinal(n: number): string {
  return ORDINALS[n] ?? `${n}th`;
}

function capitalise(text: string): string {
  return text ? text[0].toUpperCase() + text.slice(1) : text;
}

function fill(template: string, ctx: InboxClearedContext): string {
  const clears = stat(ctx, 'clears_today');
  const values: Record<string, string> = {
    Inbox: ctx.inbox,
    inbox: ctx.inboxInSentence,
    streak: String(stat(ctx, 'streak_days')),
    resolvedToday: String(stat(ctx, 'resolved_today')),
    cleared: String(stat(ctx, 'cleared_count')),
    ordinalClears: ordinal(clears),
    OrdinalClears: capitalise(ordinal(clears)),
    dayName: DAYS[ctx.day] ?? '',
  };
  return template.replace(/\{(\w+)\}/g, (_, key: string) => values[key] ?? '');
}

function pickFresh<T>(items: T[], seen: (item: T) => boolean, random: () => number): T {
  const fresh = items.filter((item) => !seen(item));
  const pool = fresh.length > 0 ? fresh : items;
  return pool[Math.min(pool.length - 1, Math.floor(random() * pool.length))];
}

/** A clear this size earns confetti on its own. */
export const CONFETTI_MIN_CLEARED = 5;

/**
 * Confetti is kept for clears that mean something: the first clear of the day
 * in this inbox, an achievement above Common, or a big clear. Every other
 * clear shows the card quietly, so a trickling inbox does not sparkle all day.
 */
export function shouldThrowConfetti(
  message: Pick<InboxClearedMessage, 'rarity'>,
  stats: InboxClearedStats | null,
  clearedCount: number,
): boolean {
  return message.rarity !== 'common'
    || stats?.clears_today === 1
    || clearedCount >= CONFETTI_MIN_CLEARED;
}

export interface ComposeOptions {
  /** Inbox the clear belongs to; rare achievements unlock once a day per inbox. */
  scopeKey: string;
  /** Viewer's local date, YYYY-MM-DD. */
  localDate: string;
  random?: () => number;
}

/**
 * Chooses the message for one clear and returns the updated memory. Pure:
 * the caller loads and saves the memory.
 */
export function composeInboxClearedMessage(
  ctx: InboxClearedContext,
  memory: InboxClearedMemory,
  { scopeKey, localDate, random = Math.random }: ComposeOptions,
): { message: InboxClearedMessage; memory: InboxClearedMemory } {
  const recent = new Set(memory.recent);
  const unlockedToday = (id: string) => memory.unlocked[`${scopeKey}:${id}`] === localDate;
  const eligible = ACHIEVEMENTS.filter((achievement) => achievement.when(ctx) && !unlockedToday(achievement.id));

  let achievement: Achievement | null = null;
  for (const rarity of RARITY_ORDER) {
    const group = eligible.filter((item) => item.rarity === rarity);
    if (group.length > 0) {
      achievement = pickFresh(group, (item) => recent.has(`a:${item.id}`), random);
      break;
    }
  }

  let message: InboxClearedMessage;
  let seenKeys: string[];
  if (achievement) {
    const variant = pickFresh(achievement.variants, (item) => recent.has(item.title), random);
    message = {
      achievementId: achievement.id,
      rarity: achievement.rarity,
      name: achievement.name,
      title: fill(variant.title, ctx),
      body: fill(variant.body, ctx),
    };
    seenKeys = [`a:${achievement.id}`, variant.title];
  } else {
    const name = pickFresh(COMMON_NAMES, (item) => recent.has(item), random);
    // "Clean sweep" as the name and the title would read twice.
    const titles = COMMON_TITLES.filter((title) => !title.toLowerCase().startsWith(name.toLowerCase()));
    const title = pickFresh(titles, (item) => recent.has(item), random);
    const line = pickFresh(COMMON_LINES.concat(TIME_LINES[partOfDay(ctx.hour)]), (item) => recent.has(item), random);
    message = { achievementId: 'everyday', rarity: 'common', name, title: fill(title, ctx), body: fill(line, ctx) };
    seenKeys = [name, title, line];
  }

  const unlocked = { ...memory.unlocked };
  if (achievement) unlocked[`${scopeKey}:${achievement.id}`] = localDate;
  for (const key of Object.keys(unlocked)) {
    if (unlocked[key] !== localDate) delete unlocked[key];
  }
  return {
    message,
    memory: { recent: memory.recent.concat(seenKeys).slice(-INBOX_CLEARED_RECENT_LIMIT), unlocked },
  };
}
