import { streamCharacterDelay } from '../../_components/streaming-text-timing';
import '../../_components/streaming-text.css';

type PreviewStream = { id: string; duration: number; elapsed: number };

/** Enhance only the inert marketing widget's already-rendered Markdown.
 * Complete message content reserves wrapping and bubble size. Decorating text
 * nodes preserves the widget's own markup, escaping and source links, without
 * changing the live customer widget or replacing its renderer.
 */
export function revealWidgetPreviewText(root: HTMLElement, stream: PreviewStream | null) {
  for (const content of root.querySelectorAll<HTMLElement>('.helpin-message-content.preview-stream')) {
    if (!content.closest('.helpin-message--streaming') || !stream) content.dataset.streaming = 'false';
  }
  if (!stream) return;
  const content = root.querySelector<HTMLElement>('.helpin-message--streaming .helpin-message-content');
  if (!content || content.dataset.previewStream === stream.id) return;
  const walker = document.createTreeWalker(content, NodeFilter.SHOW_TEXT);
  const nodes: Text[] = [];
  while (walker.nextNode()) nodes.push(walker.currentNode as Text);
  const length = nodes.reduce((total, node) => total + Array.from(node.data).length, 0);
  let index = 0;
  for (const node of nodes) {
    const fragment = document.createDocumentFragment();
    for (const character of Array.from(node.data)) {
      const glyph = document.createElement('span');
      glyph.className = 'preview-stream-character';
      glyph.textContent = character;
      glyph.style.setProperty('--stream-delay', `${streamCharacterDelay(index++, length, stream.duration, -stream.elapsed)}ms`);
      fragment.append(glyph);
    }
    node.replaceWith(fragment);
  }
  content.classList.add('preview-stream');
  content.dataset.previewStream = stream.id;
  content.dataset.streaming = 'true';
}
