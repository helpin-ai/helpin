import React, { useEffect, useRef, useState, useCallback } from 'react';
import type { Editor } from '@tiptap/core';
import { ArrowLeft } from 'lucide-react';
import { slashMenuPluginKey, type SlashMenuState } from './SlashMenuExtension';
import { slashCommands, type SlashCommand } from './slash-commands';

interface SlashMenuProps {
  editor: Editor;
  onImageInsert?: () => void;
}

const CLOSED: SlashMenuState = { open: false, from: 0, query: '', selectedIndex: 0, commandCount: 0 };

export function SlashMenu({ editor, onImageInsert }: SlashMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);
  const stateRef = useRef<SlashMenuState>({ ...CLOSED });
  const [submenu, setSubmenu] = useState<SlashCommand[] | null>(null);
  const [submenuTitle, setSubmenuTitle] = useState('');
  const [submenuIndex, setSubmenuIndex] = useState(0);

  // Read plugin state reactively via forced re-render
  const pluginState = (slashMenuPluginKey.getState(editor.state) as SlashMenuState) ?? CLOSED;
  stateRef.current = pluginState;

  // Reset submenu when menu closes
  useEffect(() => {
    if (!pluginState.open) {
      setSubmenu(null);
      setSubmenuTitle('');
      setSubmenuIndex(0);
    }
  }, [pluginState.open]);

  // Active commands: either submenu children or filtered top-level
  const activeCommands = submenu
    ? submenu
    : slashCommands.filter(
        (cmd) =>
          cmd.title.toLowerCase().includes(pluginState.query.toLowerCase()) ||
          cmd.description.toLowerCase().includes(pluginState.query.toLowerCase()),
      );

  // Keep commandCount in sync so the plugin can wrap arrow keys (top-level only)
  useEffect(() => {
    if (!submenu && pluginState.open && pluginState.commandCount !== activeCommands.length) {
      editor.view.dispatch(
        editor.state.tr.setMeta(slashMenuPluginKey, { commandCount: activeCommands.length }),
      );
    }
  }, [editor, submenu, pluginState.open, pluginState.commandCount, activeCommands.length]);

  // Re-render only when slash menu plugin state actually changes
  const [, forceUpdate] = React.useReducer((x: number) => x + 1, 0);
  const prevPluginStateRef = useRef<string>('');
  useEffect(() => {
    const handler = () => {
      const ps = slashMenuPluginKey.getState(editor.state) as SlashMenuState | undefined;
      const key = ps ? `${ps.open}|${ps.from}|${ps.query}|${ps.selectedIndex}|${ps.executeSelected}` : '';
      if (key !== prevPluginStateRef.current) {
        prevPluginStateRef.current = key;
        forceUpdate();
      }
    };
    editor.on('transaction', handler);
    return () => { editor.off('transaction', handler); };
  }, [editor]);

  const executeCommand = useCallback(
    (cmd: SlashCommand) => {
      // If command has children, open submenu instead of executing
      if (cmd.children) {
        setSubmenu(cmd.children);
        setSubmenuTitle(cmd.title);
        setSubmenuIndex(0);
        return;
      }

      // Calculate delete range from plugin state — stable regardless of extra dispatches
      const { from: menuFrom, query } = stateRef.current;
      const deleteFrom = menuFrom - 1; // position of the "/"
      const deleteTo = menuFrom + query.length; // end of typed query

      // Delete the "/" + query text, then execute the command
      editor.chain().deleteRange({ from: deleteFrom, to: deleteTo }).run();

      if (cmd.title === 'Image' && onImageInsert) {
        onImageInsert();
      } else {
        cmd.action(editor);
      }

      // Close menu after execution
      editor.view.dispatch(editor.state.tr.setMeta(slashMenuPluginKey, CLOSED));
      setSubmenu(null);
      setSubmenuTitle('');
      setSubmenuIndex(0);
    },
    [editor, onImageInsert],
  );

  const goBack = useCallback(() => {
    setSubmenu(null);
    setSubmenuTitle('');
    setSubmenuIndex(0);
  }, []);

  const handleItemClick = useCallback(
    (cmd: SlashCommand) => {
      executeCommand(cmd);
    },
    [executeCommand],
  );

  // Handle executeSelected from ProseMirror plugin (Enter key)
  useEffect(() => {
    if (!pluginState.executeSelected) return;

    // Execute the command — executeCommand will close the menu which clears the flag
    if (submenu) {
      if (submenu[submenuIndex]) {
        executeCommand(submenu[submenuIndex]);
      }
    } else if (activeCommands[pluginState.selectedIndex]) {
      executeCommand(activeCommands[pluginState.selectedIndex]);
    }
  }, [pluginState.executeSelected]);

  // Keyboard nav when in submenu mode
  useEffect(() => {
    if (!submenu) return;

    const handler = (e: KeyboardEvent) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        e.stopPropagation();
        setSubmenuIndex((i) => (i + 1) % submenu.length);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        e.stopPropagation();
        setSubmenuIndex((i) => (i - 1 + submenu.length) % submenu.length);
      } else if (e.key === 'Enter') {
        e.preventDefault();
        e.stopPropagation();
        if (submenu[submenuIndex]) executeCommand(submenu[submenuIndex]);
      } else if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        goBack();
      }
    };
    document.addEventListener('keydown', handler, true);
    return () => document.removeEventListener('keydown', handler, true);
  }, [submenu, submenuIndex, executeCommand, goBack]);

  // Scroll selected item into view — only on index change, not on initial open
  const selectedIdx = submenu ? submenuIndex : pluginState.selectedIndex;
  const prevSelectedIdx = useRef(selectedIdx);
  useEffect(() => {
    if (!pluginState.open || !menuRef.current) return;
    // Skip scroll on initial open (index is 0)
    if (prevSelectedIdx.current === selectedIdx) return;
    prevSelectedIdx.current = selectedIdx;
    const items = menuRef.current.querySelectorAll('[data-slash-item]');
    const el = items[selectedIdx];
    if (el && menuRef.current) {
      // Scroll within the menu container only, not the page
      const menuRect = menuRef.current.getBoundingClientRect();
      const itemRect = el.getBoundingClientRect();
      if (itemRect.bottom > menuRect.bottom) {
        el.scrollIntoView({ block: 'nearest', behavior: 'instant' });
      } else if (itemRect.top < menuRect.top) {
        el.scrollIntoView({ block: 'nearest', behavior: 'instant' });
      }
    }
  }, [pluginState.open, selectedIdx]);

  // Position the menu near the cursor
  useEffect(() => {
    if (!pluginState.open || !menuRef.current) return;
    const coords = editor.view.coordsAtPos(pluginState.from - 1);
    const wrapper = editor.view.dom.closest('.docs-editor-wrapper');
    if (!wrapper) return;
    const editorRect = wrapper.getBoundingClientRect();
    menuRef.current.style.top = `${coords.bottom - editorRect.top + wrapper.scrollTop + 4}px`;
    menuRef.current.style.left = `${coords.left - editorRect.left}px`;
  }, [pluginState, editor]);

  // Close on click outside
  useEffect(() => {
    if (!pluginState.open) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        editor.view.dispatch(editor.state.tr.setMeta(slashMenuPluginKey, CLOSED));
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [pluginState.open, editor]);

  if (!pluginState.open || activeCommands.length === 0) return null;

  return (
    <div
      ref={menuRef}
      className="absolute z-50 w-64 max-h-80 overflow-y-auto rounded-lg border bg-popover p-1 shadow-md"
    >
      {submenu && (
        <button
          className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm text-muted-foreground hover:bg-accent/50 mb-0.5"
          onMouseDown={(e) => e.preventDefault()}
          onClick={goBack}
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span className="font-medium">{submenuTitle}</span>
        </button>
      )}
      {activeCommands.map((cmd, i) => {
        const isSelected = submenu ? i === submenuIndex : i === pluginState.selectedIndex;
        return (
          <button
            key={cmd.title}
            data-slash-item
            className={`flex w-full items-center gap-3 rounded-md px-2 py-1.5 text-left text-sm transition-colors ${
              isSelected ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50'
            }`}
            onMouseEnter={() => {
              if (submenu) {
                setSubmenuIndex(i);
              } else {
                editor.view.dispatch(
                  editor.state.tr.setMeta(slashMenuPluginKey, { selectedIndex: i }),
                );
              }
            }}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => handleItemClick(cmd)}
          >
            <div
              className="flex h-8 w-8 items-center justify-center rounded-lg shrink-0"
              style={cmd.iconColor ? { backgroundColor: `${cmd.iconColor}20` } : undefined}
            >
              <cmd.icon
                className="h-4 w-4 shrink-0"
                style={cmd.iconColor ? { color: cmd.iconColor } : undefined}
              />
            </div>
            <div className="flex-1 min-w-0">
              <p className="font-medium">{cmd.title}</p>
            </div>
            {cmd.children && (
              <span className="text-base font-medium text-muted-foreground">›</span>
            )}
          </button>
        );
      })}
    </div>
  );
}
