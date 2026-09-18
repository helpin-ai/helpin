# Task Mermaid Saved View Design

> Historical design/implementation plan, source-compared on 2026-09-17. The
> [saved rich-text renderer](../../frontend/src/components/pm/RichTextMentionContent.tsx)
> now implements direct `pre > code` detection with case-insensitive language
> classes and passes `textContent` to the diagram component. It also recognizes
> `language-nwdiag` and `language-svg`; ordinary code blocks retain the generic
> rendering path. The [regression test](../../frontend/src/components/pm/__tests__/RichTextMentionContent.test.tsx)
> covers source preservation for all three diagram types using mocked renderers.
> It does not validate actual diagram layout. Unchecked steps and expected failures
> below describe the original plan, not missing implementation or current test
> results. No browser rendering or build was rerun for this documentation review.

## Problem

Task descriptions use the shared TipTap editor while editing and `RichTextMentionContent` after editing. The editor's custom code-block node view renders Mermaid code blocks as diagrams, but the saved-view renderer recursively reproduces the stored `<pre><code class="language-mermaid">` HTML. The Mermaid language metadata survives saving, but the saved view does not interpret it.

## Design

Extend `RichTextMentionContent` at its HTML-rendering boundary. When it encounters a `pre` element whose direct `code` child is marked as Mermaid, render the existing `MermaidBlock` with the code element's text content. Leave all non-Mermaid code blocks and other rich-text elements unchanged.

Recognize the standard TipTap `language-mermaid` class case-insensitively. This preserves Mermaid source as the persisted representation, reuses the editor's established renderer and error state, and avoids mounting a second read-only TipTap editor.

## Testing

Add a component regression test that supplies saved TipTap HTML and mocks `MermaidBlock` so it can assert that the read-only task renderer receives the original diagram source. Retain the existing rich-text tests to prove ordinary markup behavior remains intact.
