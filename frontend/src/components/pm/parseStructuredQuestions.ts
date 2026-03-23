import type { StructuredQuestion } from '@/lib/pmTypes';

const questionsRegex = /<questions>([\s\S]*?)<\/questions>/i;

export interface ParsedQuestions {
  questions: StructuredQuestion[];
  surroundingText: string;
}

/**
 * Parse <questions> XML from a message. Returns null if no block found or XML is malformed.
 */
export function parseStructuredQuestions(content: string): ParsedQuestions | null {
  const match = questionsRegex.exec(content);
  if (!match) return null;

  const xmlString = `<questions>${match[1]}</questions>`;
  const surroundingText = content.replace(questionsRegex, '').trim();

  try {
    const doc = new DOMParser().parseFromString(xmlString, 'text/xml');
    if (doc.querySelector('parsererror')) return null;

    const questions: StructuredQuestion[] = [];
    const root = doc.documentElement;
    const childElements = Array.from(root.children);

    // Collect orphan <option> tags that are direct children of <questions>
    // (LLM sometimes puts a shared "Other" option outside any <question>)
    const orphanOptions = parseOptionElements(
      Array.from(root.children).filter(
        (el) => el.tagName.toLowerCase() === 'option',
      ),
    );

    for (let index = 0; index < childElements.length; index += 1) {
      const qEl = childElements[index];
      if (qEl.tagName.toLowerCase() !== 'question') continue;

      const id = qEl.getAttribute('id') ?? `q${questions.length + 1}`;
      let text = extractQuestionText(qEl);

      let options = parseOptionElements(qEl.querySelectorAll('option'));
      if (options.length === 0) {
        const nextEl = childElements[index + 1];
        if (nextEl?.tagName.toLowerCase() === 'options') {
          options = parseOptionElements(nextEl.querySelectorAll('option'));
          index += 1;
        }
      }

      if (options.length === 0) {
        const fallback = parseQuestionBodyFallback(qEl.textContent ?? '');
        if (fallback.options.length > 0 && fallback.text) {
          // Use the fallback's split text (before bullets) instead of the
          // full textContent which includes the bullet items.
          text = fallback.text;
        } else if (!text) {
          text = fallback.text;
        }
        options = fallback.options;
      }

      // Append orphan options (e.g. a shared "Other" option) if the question
      // has no options of its own or is missing a freetext option.
      if (options.length === 0 && orphanOptions.length > 0) {
        options = [...orphanOptions];
      } else if (options.length > 0 && orphanOptions.length > 0) {
        const hasFreetextOption = options.some((o) => o.freetext);
        if (!hasFreetextOption) {
          const freetextOrphans = orphanOptions.filter((o) => o.freetext);
          options = [...options, ...freetextOrphans];
        }
      }

      if (!text || options.length === 0) continue;

      questions.push({ id, text, options });
    }

    if (questions.length === 0) return null;
    return { questions, surroundingText };
  } catch {
    return null;
  }
}

function extractQuestionText(qEl: Element): string {
  const textAttr = qEl.getAttribute('text')?.trim();
  if (textAttr) return textAttr;

  const textEl = qEl.querySelector('text');
  const nestedText = textEl?.textContent?.trim();
  if (nestedText) return nestedText;

  const directText = Array.from(qEl.childNodes)
    .filter((node) => node.nodeType === Node.TEXT_NODE)
    .map((node) => node.textContent?.trim() ?? '')
    .filter(Boolean)
    .join(' ')
    .trim();
  if (directText) return directText;

  return qEl.textContent?.trim() ?? '';
}

function parseOptionElements(optionNodes: Iterable<Element>): StructuredQuestion['options'] {
  return Array.from(optionNodes).map((opt) => ({
    value: opt.getAttribute('value') ?? '',
    label: opt.textContent?.trim() ?? '',
    freetext: opt.getAttribute('freetext') === 'true' || undefined,
  })).filter((option) => option.label.length > 0);
}

function parseQuestionBodyFallback(raw: string): StructuredQuestion {
  const normalized = raw.replace(/\s+/g, ' ').trim();
  if (!normalized) {
    return { id: '', text: '', options: [] };
  }

  const bulletMatches = Array.from(normalized.matchAll(/\s[-•]\s+/g));
  if (bulletMatches.length === 0) {
    return { id: '', text: normalized, options: [] };
  }

  const questionText = normalized.slice(0, bulletMatches[0].index).trim();
  const options = bulletMatches.map((match, index) => {
    const start = (match.index ?? 0) + match[0].length;
    const end = index + 1 < bulletMatches.length ? bulletMatches[index + 1].index : normalized.length;
    const label = normalized.slice(start, end).trim();
    const freetext = /^other\b/i.test(label) || /please specify/i.test(label);
    return {
      value: optionValueFromLabel(label, index),
      label,
      freetext: freetext || undefined,
    };
  }).filter((option) => option.label.length > 0);

  return {
    id: '',
    text: questionText,
    options,
  };
}

function optionValueFromLabel(label: string, index: number): string {
  if (/^other\b/i.test(label)) {
    return 'other';
  }

  const slug = label
    .toLowerCase()
    .replace(/['"]/g, '')
    .replace(/\([^)]*\)/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

  return slug || `option_${index + 1}`;
}
