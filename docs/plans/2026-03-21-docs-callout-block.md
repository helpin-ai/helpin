# Docs editor callout block implementation plan

This historical plan explains the initial callout implementation. The editor and
server now use semantic variants; read the current notes before copying the old
color-only snippets or treating unchecked steps as missing features.

## Current implementation and limits

Source-compared on 2026-09-18. No browser rendering or application tests were run.

- [CalloutExtension](../../frontend/src/components/docs/CalloutExtension.ts)
  supports `info`, `warning`, `tip`, `danger`, and `success`, with legacy aliases:
  blue maps to info, yellow to warning, green to tip, red to danger, and grey to
  info. Missing or unknown variants default to info, not grey.
- The [Go renderer](../../server/internal/tiptap/html.go) uses the same mapping
  and emits semantic `docs-callout--*` classes on an aside. The
  [slash commands](../../frontend/src/components/docs/slash-commands.ts) insert
  semantic variants, and [CalloutNodeView](../../frontend/src/components/docs/CalloutNodeView.tsx)
  renders the corresponding editor presentation.
- The inspected [widget stylesheet](../../packages/widget-core/src/styles/widget.css)
  and [help-center stylesheet](../../help-center/src/app.css) still define the
  old blue/green/grey/red/yellow variant selectors. No semantic variant selectors
  were found in CSS under those packages. Their generic callout rule still
  applies, but these old variant selectors do not match newly rendered semantic
  classes. Consistent public variant colors are therefore not established.
- Keyboard handling includes splitting blocks, exiting after trailing empty
  paragraphs, and removing an empty callout with Backspace/Delete. The original
  simple manual checklist does not describe all current conditions.
- [Renderer tests](../../server/internal/tiptap/html_test.go) exist. The filtered
  TypeScript command below is not a reliable successful-build check: filtering
  diagnostics can hide unrelated compiler failures. Use the current full
  frontend typecheck/build instructions and inspect the process exit status.

Original commit commands and expected test outcomes are implementation history,
not actions or results of this documentation review. The styling mismatch above
is recorded for follow-up; this task changes documentation only.

## Original implementation sequence

**Goal:** Add a custom callout block node to the Docs editor with 5 color variants (blue, green, grey, red, yellow), Go server-side rendering, and Help Center styling.

**Architecture:** Create a custom Tiptap Node extension (`callout`) with a React NodeView for variant-specific styling and editable nested content. Add a slash menu entry. Extend the Go HTML renderer to output `<aside class="docs-callout docs-callout--{variant}">`. Add CSS to the editor, widget, and help-center.

**Tech Stack:** Tiptap 3 (custom Node + ReactNodeViewRenderer), Go (tiptap/html.go renderer), CSS

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `frontend/src/components/docs/CalloutExtension.ts` | Tiptap Node extension: name, attrs, parseHTML, renderHTML, commands |
| Create | `frontend/src/components/docs/CalloutNodeView.tsx` | React NodeView: variant picker, colored container, editable content |
| Modify | `frontend/src/components/docs/slash-commands.ts` | Add Callout slash menu entries |
| Modify | `frontend/src/components/docs/DocsEditor.tsx` | Register CalloutExtension |
| Modify | `frontend/src/index.css` | Editor callout styles |
| Modify | `server/internal/tiptap/html.go` | Add `callout` case to renderNode |
| Modify | `server/internal/tiptap/html_test.go` | Add callout render test |
| Modify | `packages/widget-core/src/styles/widget.css` | Widget callout styles |
| Modify | `help-center/src/app.css` | Help center callout styles |

---

### Task 1: Add callout render case to Go HTML renderer (with test)

**Files:**
- Modify: `server/internal/tiptap/html_test.go`
- Modify: `server/internal/tiptap/html.go`

- [ ] **Step 1: Write the failing test**

Add to `server/internal/tiptap/html_test.go`:

```go
func TestRenderHTML_Callout(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"callout","attrs":{"variant":"yellow"},"content":[{"type":"paragraph","content":[{"type":"text","text":"This is a warning."}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<aside class="docs-callout docs-callout--yellow"`) {
		t.Errorf("expected callout aside with variant class, got: %s", got)
	}
	if !strings.Contains(got, "This is a warning.") {
		t.Errorf("expected callout content, got: %s", got)
	}
}

func TestRenderHTML_CalloutDefaultVariant(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"callout","content":[{"type":"paragraph","content":[{"type":"text","text":"No variant."}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `docs-callout--grey`) {
		t.Errorf("expected default grey variant, got: %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/tiptap/ -run TestRenderHTML_Callout -v`
Expected: FAIL — callout renders as empty (unknown node fallback)

- [ ] **Step 3: Add callout case to renderNode**

In `server/internal/tiptap/html.go`, add a new case in the `renderNode` switch statement, after the `"blockquote"` case:

```go
	case "callout":
		variant := strAttr(n.Attrs, "variant")
		if variant == "" {
			variant = "grey"
		}
		// Only allow known variants to prevent class injection.
		switch variant {
		case "blue", "green", "grey", "red", "yellow":
		default:
			variant = "grey"
		}
		fmt.Fprintf(b, "<aside class=\"docs-callout docs-callout--%s\" data-callout-variant=\"%s\">\n", variant, variant)
		renderChildren(b, n)
		b.WriteString("</aside>\n")
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/tiptap/ -run TestRenderHTML_Callout -v`
Expected: PASS

- [ ] **Step 5: Verify server compiles**

Run: `cd server && go build ./cmd/api`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add server/internal/tiptap/html.go server/internal/tiptap/html_test.go
git commit -m "feat: add callout block rendering to Go HTML renderer"
```

---

### Task 2: Create Tiptap CalloutExtension

**Files:**
- Create: `frontend/src/components/docs/CalloutExtension.ts`

- [ ] **Step 1: Create the extension**

```ts
import { Node, mergeAttributes } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { CalloutNodeView } from './CalloutNodeView';

export type CalloutVariant = 'blue' | 'green' | 'grey' | 'red' | 'yellow';

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    callout: {
      setCallout: (attrs?: { variant?: CalloutVariant }) => ReturnType;
    };
  }
}

export const CalloutExtension = Node.create({
  name: 'callout',
  group: 'block',
  content: 'block+',
  defining: true,

  addAttributes() {
    return {
      variant: {
        default: 'grey',
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-callout-variant') || 'grey',
        renderHTML: (attrs) => ({ 'data-callout-variant': attrs.variant }),
      },
    };
  },

  parseHTML() {
    return [
      { tag: 'aside[data-callout-variant]' },
      { tag: 'div[data-helpin-callout]', getAttrs: (el) => ({ variant: (el as HTMLElement).getAttribute('data-helpin-callout') || 'grey' }) },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    const v = HTMLAttributes['data-callout-variant'] ?? 'grey';
    return ['aside', mergeAttributes(HTMLAttributes, { class: `docs-callout docs-callout--${v}` }), 0];
  },

  addNodeView() {
    return ReactNodeViewRenderer(CalloutNodeView);
  },

  addCommands() {
    return {
      setCallout:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: { variant: attrs?.variant ?? 'grey' },
            content: [{ type: 'paragraph' }],
          }),
    };
  },
});
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/docs/CalloutExtension.ts
git commit -m "feat: add Tiptap callout node extension with variant attrs"
```

---

### Task 3: Create CalloutNodeView React component

**Files:**
- Create: `frontend/src/components/docs/CalloutNodeView.tsx`

- [ ] **Step 1: Create the NodeView component**

```tsx
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/react';
import { AlertCircle, CheckCircle2, Info, AlertTriangle, MessageSquare } from 'lucide-react';
import type { CalloutVariant } from './CalloutExtension';

const VARIANT_STYLES: Record<CalloutVariant, { bg: string; border: string; icon: string }> = {
  blue: { bg: 'bg-blue-50 dark:bg-blue-950/30', border: 'border-blue-300 dark:border-blue-700', icon: 'text-blue-600 dark:text-blue-400' },
  green: { bg: 'bg-green-50 dark:bg-green-950/30', border: 'border-green-300 dark:border-green-700', icon: 'text-green-600 dark:text-green-400' },
  grey: { bg: 'bg-muted/50', border: 'border-border', icon: 'text-muted-foreground' },
  red: { bg: 'bg-red-50 dark:bg-red-950/30', border: 'border-red-300 dark:border-red-700', icon: 'text-red-600 dark:text-red-400' },
  yellow: { bg: 'bg-yellow-50 dark:bg-yellow-950/30', border: 'border-yellow-300 dark:border-yellow-700', icon: 'text-yellow-600 dark:text-yellow-400' },
};

const VARIANT_ICONS: Record<CalloutVariant, typeof Info> = {
  blue: Info,
  green: CheckCircle2,
  grey: MessageSquare,
  red: AlertCircle,
  yellow: AlertTriangle,
};

const VARIANTS: CalloutVariant[] = ['blue', 'green', 'grey', 'red', 'yellow'];

export function CalloutNodeView({ node, updateAttributes, editor }: NodeViewProps) {
  const variant = (node.attrs.variant as CalloutVariant) || 'grey';
  const styles = VARIANT_STYLES[variant];
  const Icon = VARIANT_ICONS[variant];
  const editable = editor.isEditable;

  return (
    <NodeViewWrapper>
      <aside className={`my-3 flex gap-3 rounded-lg border-l-4 p-4 ${styles.bg} ${styles.border}`}>
        <div className="flex flex-col items-center gap-1 pt-0.5">
          <Icon className={`h-5 w-5 shrink-0 ${styles.icon}`} />
          {editable && (
            <div className="flex flex-col gap-0.5 mt-1">
              {VARIANTS.map((v) => (
                <button
                  key={v}
                  type="button"
                  className={`h-3 w-3 rounded-full border transition-transform ${
                    v === variant ? 'scale-125 ring-1 ring-offset-1 ring-current' : 'opacity-50 hover:opacity-100'
                  }`}
                  style={{
                    backgroundColor:
                      v === 'blue' ? '#3b82f6' :
                      v === 'green' ? '#22c55e' :
                      v === 'grey' ? '#9ca3af' :
                      v === 'red' ? '#ef4444' :
                      '#eab308',
                  }}
                  onClick={() => updateAttributes({ variant: v })}
                  title={v.charAt(0).toUpperCase() + v.slice(1)}
                />
              ))}
            </div>
          )}
        </div>
        <div className="flex-1 min-w-0">
          <NodeViewContent />
        </div>
      </aside>
    </NodeViewWrapper>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/docs/CalloutNodeView.tsx
git commit -m "feat: add CalloutNodeView with variant picker and colored styling"
```

---

### Task 4: Register extension and add slash menu entries

**Files:**
- Modify: `frontend/src/components/docs/DocsEditor.tsx`
- Modify: `frontend/src/components/docs/slash-commands.ts`

- [ ] **Step 1: Add callout import to DocsEditor.tsx**

Add after the existing table/slash imports:

```ts
import { CalloutExtension } from './CalloutExtension';
```

- [ ] **Step 2: Register extension in useEditor**

Add `CalloutExtension` to the extensions array, after `SlashMenuExtension`:

```ts
CalloutExtension,
```

- [ ] **Step 3: Add callout to slash commands**

In `frontend/src/components/docs/slash-commands.ts`, add the import:

```ts
import { MessageSquareWarning } from 'lucide-react';
```

Add the callout entry to the `slashCommands` array, after the Divider entry:

```ts
  {
    title: 'Callout',
    description: 'Highlighted note or warning',
    icon: MessageSquareWarning,
    action: (editor) => editor.chain().focus().setCallout({ variant: 'grey' }).run(),
  },
```

- [ ] **Step 4: Verify it compiles**

Run: `cd frontend && npx tsc --noEmit 2>&1 | grep -E "Callout|slash-commands|DocsEditor" | head -10`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/docs/DocsEditor.tsx frontend/src/components/docs/slash-commands.ts
git commit -m "feat: register callout extension and add slash menu entry"
```

---

### Task 5: Add callout CSS to editor, widget, and help center

**Files:**
- Modify: `frontend/src/index.css`
- Modify: `packages/widget-core/src/styles/widget.css`
- Modify: `help-center/src/app.css`

- [ ] **Step 1: Add editor callout styles**

In `frontend/src/index.css`, add after the existing `.tiptap .selectedCell` rule:

```css
.tiptap .docs-callout {
  margin: 0.75rem 0;
}
```

- [ ] **Step 2: Add widget callout styles**

In `packages/widget-core/src/styles/widget.css`, add after the `.helpin-article-content hr` rule:

```css
.helpin-article-content .docs-callout {
  border-left: 4px solid;
  border-radius: 6px;
  padding: 12px 14px;
  margin: 0 0 1em;
}
.helpin-article-content .docs-callout--blue {
  border-color: rgba(59, 130, 246, 0.5);
  background: rgba(59, 130, 246, 0.06);
}
.helpin-article-content .docs-callout--green {
  border-color: rgba(34, 197, 94, 0.5);
  background: rgba(34, 197, 94, 0.06);
}
.helpin-article-content .docs-callout--grey {
  border-color: rgba(156, 163, 175, 0.5);
  background: rgba(156, 163, 175, 0.06);
}
.helpin-article-content .docs-callout--red {
  border-color: rgba(239, 68, 68, 0.5);
  background: rgba(239, 68, 68, 0.06);
}
.helpin-article-content .docs-callout--yellow {
  border-color: rgba(234, 179, 8, 0.5);
  background: rgba(234, 179, 8, 0.06);
}
.helpin-article-content .docs-callout > *:last-child {
  margin-bottom: 0;
}
```

- [ ] **Step 3: Add help center callout styles**

In `help-center/src/app.css`, add after the existing `.hc-prose blockquote` styles (find them first with grep):

```css
.hc-prose .docs-callout {
  border-left: 4px solid;
  border-radius: 0.5rem;
  padding: 1rem 1.25rem;
  margin: 1.25rem 0;
}
.hc-prose .docs-callout--blue {
  border-color: oklch(0.62 0.19 255);
  background: oklch(0.62 0.19 255 / 0.06);
}
.hc-prose .docs-callout--green {
  border-color: oklch(0.72 0.19 155);
  background: oklch(0.72 0.19 155 / 0.06);
}
.hc-prose .docs-callout--grey {
  border-color: oklch(0.71 0.01 260);
  background: oklch(0.71 0.01 260 / 0.06);
}
.hc-prose .docs-callout--red {
  border-color: oklch(0.63 0.24 25);
  background: oklch(0.63 0.24 25 / 0.06);
}
.hc-prose .docs-callout--yellow {
  border-color: oklch(0.79 0.17 85);
  background: oklch(0.79 0.17 85 / 0.06);
}
.hc-prose .docs-callout > *:last-child {
  margin-bottom: 0;
}
```

Note: The help-center uses Tailwind v4 with oklch colors, while the widget uses rgba for broader compatibility.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/index.css packages/widget-core/src/styles/widget.css help-center/src/app.css
git commit -m "feat: add callout block styles to editor, widget, and help center"
```

---

### Task 6: Manual verification

- [ ] **Step 1:** Open a doc in the editor
- [ ] **Step 2:** Type `/callout` — should show "Callout" in slash menu, press Enter to insert
- [ ] **Step 3:** Callout should appear as a blue-highlighted box with info icon and editable content
- [ ] **Step 4:** Click the color dots on the left to switch variants (blue, green, grey, red, yellow)
- [ ] **Step 5:** Type text inside the callout — should be fully editable
- [ ] **Step 6:** Save the document, reload — callout should persist with correct variant
- [ ] **Step 7:** View the document in the Help Center (`/help-center`) — callout should render as a colored aside
- [ ] **Step 8:** Test pressing Enter inside callout creates new paragraphs within the callout
- [ ] **Step 9:** Test pressing Backspace at start of empty callout removes it
