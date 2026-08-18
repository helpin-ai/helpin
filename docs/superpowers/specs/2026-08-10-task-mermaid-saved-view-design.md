# Task Mermaid Saved View Design

## Problem

Task descriptions use the shared TipTap editor while editing and `RichTextMentionContent` after editing. The editor's custom code-block node view renders Mermaid code blocks as diagrams, but the saved-view renderer recursively reproduces the stored `<pre><code class="language-mermaid">` HTML. The Mermaid language metadata survives saving, but the saved view does not interpret it.

## Design

Extend `RichTextMentionContent` at its HTML-rendering boundary. When it encounters a `pre` element whose direct `code` child is marked as Mermaid, render the existing `MermaidBlock` with the code element's text content. Leave all non-Mermaid code blocks and other rich-text elements unchanged.

Recognize the standard TipTap `language-mermaid` class case-insensitively. This preserves Mermaid source as the persisted representation, reuses the editor's established renderer and error state, and avoids mounting a second read-only TipTap editor.

## Testing

Add a component regression test that supplies saved TipTap HTML and mocks `MermaidBlock` so it can assert that the read-only task renderer receives the original diagram source. Retain the existing rich-text tests to prove ordinary markup behavior remains intact.
