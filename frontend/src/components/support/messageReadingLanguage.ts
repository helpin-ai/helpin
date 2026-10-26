import { francAll } from 'franc-min';

// Only unambiguous language codes; script variants keep manual Translate.
const readingLanguageCodes: Record<string, string> = {
  en: 'eng', de: 'deu', fr: 'fra', es: 'spa', it: 'ita', pt: 'por', nl: 'nld',
  pl: 'pol', uk: 'ukr', ru: 'rus', tr: 'tur', he: 'heb', hi: 'hin', bn: 'ben',
  ur: 'urd', ja: 'jpn', ko: 'kor', vi: 'vie', th: 'tha', id: 'ind', sv: 'swe',
  da: 'dan', fi: 'fin', cs: 'ces', ro: 'ron', el: 'ell',
};

// English regional tags still represent English for display translation. Do not
// collapse arbitrary script variants (for example zh-Hant versus zh-Hans).
export function normalizeReadingLanguage(language: string): string {
  const normalized = language.trim().toLowerCase().replaceAll('_', '-');
  return /^en(?:-[a-z]{2}|-[0-9]{3})?$/.test(normalized) ? 'en' : normalized;
}

// This is a display hint only: never choose a send language or trigger a model
// request from it. Unknown, short and mixed text keeps manual Translate available.
export function isMessageInReadingLanguage(content: string, readingLanguage: string): boolean {
  const reading = normalizeReadingLanguage(readingLanguage);
  // The local detector cannot distinguish regional/script variants.
  const expected = readingLanguageCodes[reading];
  if (!expected) return false;
  const text = authoredText(content);
  const letters = text.match(/\p{L}/gu)?.length ?? 0;
  if (letters < 30 || text.length > 8000) return false;
  const matches = (sample: string) => {
    const [best, next] = francAll(sample, { minLength: 30 });
    // Scores are relative distances, not probabilities. Require a clear lead.
    return best?.[0] === expected && best[1] - (next?.[1] ?? 0) >= 0.05;
  };
  if (!matches(text)) return false;
  // Do not let a long English paragraph hide a foreign-language paragraph.
  if (!text.split(/\n+/).every(part => (part.match(/\p{L}/gu)?.length ?? 0) < 30 || matches(part))) return false;
  // Mixed-language sentences can also appear within a single paragraph.
  return text.split(/[.!?。！？]+/u).every(part => {
    if ((part.match(/\p{L}/gu)?.length ?? 0) < 30) return true;
    const [best, next] = francAll(part, { minLength: 30 });
    return !best || best[0] === expected || best[0] === 'und' || best[1] - (next?.[1] ?? 0) < 0.05;
  });
}

// For a conversation already known to be in the reading language. Only clear
// evidence counts: the detector often misreads short English lines and product
// names, so a weak signal must not bring Translate back. Unknown reading
// languages keep manual Translate available.
export function isMessageInOtherLanguage(content: string, readingLanguage: string): boolean {
  const expected = readingLanguageCodes[normalizeReadingLanguage(readingLanguage)];
  if (!expected) return true;
  const text = authoredText(content);
  return [text, ...text.split(/\n+/)].some(part => {
    const letters = part.match(/\p{L}/gu)?.length ?? 0;
    if (!letters) return false;
    // A language missing from the ranking is written in another script. A
    // score of 0 is not: very short text ties many languages for last place.
    const ranked = francAll(part, { minLength: 1 }).find(([code]) => code === expected);
    if (!ranked) return true;
    // Short text only shows its script; longer text also shows its language.
    return letters >= 30 && ranked[1] < 0.8;
  });
}

function authoredText(content: string): string {
  return content
    .replace(/```[\s\S]*?```/g, '')
    .replace(/`[^`\n]*`/g, '')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/https?:\/\/\S+|[\w.+-]+@[\w.-]+\.[a-z]{2,}/gi, '')
    .replace(/^>.*$/gm, '')
    .trim();
}
