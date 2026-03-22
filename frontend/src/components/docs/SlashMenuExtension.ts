import { Extension } from '@tiptap/core';
import { Plugin, PluginKey } from '@tiptap/pm/state';

export interface SlashMenuState {
  open: boolean;
  from: number;
  query: string;
  selectedIndex: number;
  commandCount: number;
  executeSelected?: boolean;
}

const CLOSED: SlashMenuState = { open: false, from: 0, query: '', selectedIndex: 0, commandCount: 0 };

export const slashMenuPluginKey = new PluginKey('slashMenu');

export const SlashMenuExtension = Extension.create({
  name: 'slashMenu',

  addProseMirrorPlugins() {
    return [
      new Plugin({
        key: slashMenuPluginKey,
        state: {
          init(): SlashMenuState {
            return CLOSED;
          },
          apply(tr, prev): SlashMenuState {
            const meta = tr.getMeta(slashMenuPluginKey);
            if (meta !== undefined) return { ...prev, ...meta };
            if (!prev.open) return prev;
            // If the selection moved away from the slash position, close
            const { from } = tr.selection;
            if (from < prev.from) return CLOSED;
            // Update query from text between slash and cursor
            const text = tr.doc.textBetween(prev.from, from, '\0', '\0');
            // Only reset selectedIndex when the query actually changed
            const idx = text !== prev.query ? 0 : prev.selectedIndex;
            return { ...prev, query: text, selectedIndex: idx };
          },
        },
        props: {
          handleKeyDown(view, event) {
            const state = slashMenuPluginKey.getState(view.state) as SlashMenuState;
            if (!state?.open) return false;

            if (event.key === 'Escape') {
              view.dispatch(view.state.tr.setMeta(slashMenuPluginKey, CLOSED));
              return true;
            }
            if (event.key === 'ArrowDown') {
              const next = (state.selectedIndex + 1) % Math.max(state.commandCount, 1);
              view.dispatch(view.state.tr.setMeta(slashMenuPluginKey, { selectedIndex: next }));
              return true;
            }
            if (event.key === 'ArrowUp') {
              const count = Math.max(state.commandCount, 1);
              const next = (state.selectedIndex - 1 + count) % count;
              view.dispatch(view.state.tr.setMeta(slashMenuPluginKey, { selectedIndex: next }));
              return true;
            }
            if (event.key === 'Enter') {
              view.dispatch(view.state.tr.setMeta(slashMenuPluginKey, { executeSelected: true }));
              return true;
            }
            return false;
          },
          handleTextInput(view, _from, _to, text) {
            if (text !== '/') return false;
            // Only trigger at start of empty text block or after whitespace
            const { $from } = view.state.selection;
            const textBefore = $from.parent.textBetween(0, $from.parentOffset, '\0', '\0');
            if (textBefore.length === 0 || textBefore.endsWith(' ')) {
              // Schedule opening after the character is inserted
              setTimeout(() => {
                const { from: currentFrom } = view.state.selection;
                const tr = view.state.tr.setMeta(slashMenuPluginKey, {
                  open: true,
                  from: currentFrom,
                  query: '',
                  selectedIndex: 0,
                });
                view.dispatch(tr);
              });
            }
            return false;
          },
        },
      }),
    ];
  },
});
