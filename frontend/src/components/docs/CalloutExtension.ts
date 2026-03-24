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

  addKeyboardShortcuts() {
    const findCalloutDepth = (editor: any): number => {
      const { $from } = editor.state.selection;
      for (let d = $from.depth; d > 0; d--) {
        if ($from.node(d).type.name === this.name) return d;
      }
      return -1;
    };

    const deleteEmptyCallout = (editor: any, requireCursorAtStart: boolean): boolean => {
      const depth = findCalloutDepth(editor);
      if (depth < 0) return false;
      const { $from } = editor.state.selection;
      const calloutNode = $from.node(depth);
      if (
        calloutNode.childCount === 1 &&
        calloutNode.firstChild?.type.name === 'paragraph' &&
        calloutNode.firstChild?.content.size === 0 &&
        (!requireCursorAtStart || $from.parentOffset === 0)
      ) {
        const calloutFrom = $from.before(depth);
        const calloutTo = $from.after(depth);
        return editor.chain()
          .command(({ tr }: any) => {
            tr.replaceWith(calloutFrom, calloutTo, editor.state.schema.nodes.paragraph.create());
            return true;
          })
          .focus()
          .run();
      }
      return false;
    };

    return {
      Enter: ({ editor }) => {
        if (!editor.isActive('callout')) return false;
        const depth = findCalloutDepth(editor);
        if (depth < 0) return false;

        const { $from } = editor.state.selection;
        const calloutNode = $from.node(depth);
        const childIndex = $from.index(depth);
        const isLastChild = childIndex === calloutNode.childCount - 1;
        const currentChild = depth + 1 <= $from.depth ? $from.node(depth + 1) : null;
        const isEmpty = currentChild?.type.name === 'paragraph' && currentChild?.content.size === 0;

        // Exit on 3rd Enter: if the last TWO children are empty paragraphs
        if (isLastChild && isEmpty && calloutNode.childCount >= 3) {
          const prevChild = calloutNode.child(childIndex - 1);
          if (prevChild.type.name === 'paragraph' && prevChild.content.size === 0) {
            const after = $from.after(depth);
            return editor.chain()
              .command(({ tr }) => {
                const currentFrom = $from.before(depth + 1);
                const currentTo = $from.after(depth + 1);
                tr.delete(currentFrom, currentTo);
                const newResolved = tr.doc.resolve(currentFrom - 1);
                const prevFrom = newResolved.before(depth + 1);
                const prevTo = newResolved.after(depth + 1);
                tr.delete(prevFrom, prevTo);
                return true;
              })
              .insertContentAt(after - 4, { type: 'paragraph' })
              .focus()
              .run();
          }
        }

        return editor.chain().splitBlock().run();
      },
      Backspace: ({ editor }) => {
        if (!editor.isActive('callout')) return false;
        return deleteEmptyCallout(editor, true);
      },
      Delete: ({ editor }) => {
        if (!editor.isActive('callout')) return false;
        return deleteEmptyCallout(editor, false);
      },
    };
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
