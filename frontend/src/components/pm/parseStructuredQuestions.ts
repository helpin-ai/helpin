import type { StructuredQuestion } from '@/lib/pmTypes';

const questionsRegex = /<questions>([\s\S]*?)<\/questions>/;

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
    const qElements = doc.querySelectorAll('question');

    for (const qEl of qElements) {
      const id = qEl.getAttribute('id') ?? `q${questions.length + 1}`;
      const textEl = qEl.querySelector('text');
      if (!textEl?.textContent) continue;

      const options = Array.from(qEl.querySelectorAll('option')).map((opt) => ({
        value: opt.getAttribute('value') ?? '',
        label: opt.textContent?.trim() ?? '',
        freetext: opt.getAttribute('freetext') === 'true' || undefined,
      }));

      if (options.length === 0) continue;

      questions.push({ id, text: textEl.textContent.trim(), options });
    }

    if (questions.length === 0) return null;
    return { questions, surroundingText };
  } catch {
    return null;
  }
}
