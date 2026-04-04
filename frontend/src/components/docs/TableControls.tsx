import { useEffect, useRef, useState, useCallback } from 'react';
import type { Editor } from '@tiptap/core';
import {
  ArrowLeft02Icon,
  ArrowRight02Icon,
  ArrowUp02Icon,
  ArrowDown02Icon,
  Copy01Icon,
  Delete01Icon,
  CancelCircleIcon,
  PlusSignIcon,
  MoreHorizontalIcon,
  MoreVerticalIcon,
  EraserIcon,
} from '@/lib/icons';

interface TableControlsProps {
  editor: Editor;
}

interface MenuState {
  type: 'column' | 'row' | 'table';
  x: number;
  y: number;
}

interface TableHover {
  tableEl: HTMLTableElement;
  tableRect: DOMRect;
  colIndex: number;
  rowIndex: number;
  cellRect: DOMRect; // hovered cell for column positioning
  rowRect: DOMRect;  // hovered row for row positioning
}

export function TableControls({ editor }: TableControlsProps) {
  const [hover, setHover] = useState<TableHover | null>(null);
  const [menu, setMenu] = useState<MenuState | null>(null);
  const [tableSelected, setTableSelected] = useState(false);
  const [activeCellRect, setActiveCellRect] = useState<DOMRect | null>(null);
  const [mouseOverTable, setMouseOverTable] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const controlsRef = useRef<HTMLDivElement>(null);
  const clearTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const colBarRef = useRef<HTMLButtonElement>(null);
  const rowBarRef = useRef<HTMLButtonElement>(null);
  const cellHighlightRef = useRef<HTMLDivElement>(null);

  const getWrapper = useCallback(() => {
    return editor.view.dom.closest('.docs-editor-wrapper') as HTMLElement | null;
  }, [editor]);

  const toLocal = useCallback((r: DOMRect) => {
    const wrapper = getWrapper();
    if (!wrapper) return r;
    const wr = wrapper.getBoundingClientRect();
    return new DOMRect(r.left - wr.left, r.top - wr.top + wrapper.scrollTop, r.width, r.height);
  }, [getWrapper]);

  // Listen to mousemove on the wrapper — find table under mouse, independent of cursor
  useEffect(() => {
    const wrapper = getWrapper();
    if (!wrapper) return;

    const handleMouseMove = (e: MouseEvent) => {
      const target = e.target as HTMLElement;

      // If over our controls or menu, keep hover alive
      if (controlsRef.current?.contains(target) || menuRef.current?.contains(target)) {
        if (clearTimerRef.current) {
          clearTimeout(clearTimerRef.current);
          clearTimerRef.current = null;
        }
        return;
      }

      // Find table cell under mouse
      const cell = target.closest('td, th') as HTMLTableCellElement | null;
      const table = cell?.closest('table') as HTMLTableElement | null;

      if (!cell || !table) {
        setMouseOverTable(false);
        // Not over a table — start delayed clear
        if (!clearTimerRef.current) {
          clearTimerRef.current = setTimeout(() => {
            setHover(null);
            clearTimerRef.current = null;
          }, 600);
        }
        return;
      }

      setMouseOverTable(true);
      // Cancel pending clear
      if (clearTimerRef.current) {
        clearTimeout(clearTimerRef.current);
        clearTimerRef.current = null;
      }

      const row = cell.closest('tr')!;
      const rows = Array.from(table.querySelectorAll('tr'));
      const cells = Array.from(row.children);
      const colIndex = cells.indexOf(cell);
      const rowIndex = rows.indexOf(row);

      // Use the header cell in the same column for column dot positioning
      const headerRow = rows[0];
      const headerCell = headerRow?.children[colIndex] as HTMLElement | undefined;

      setHover({
        tableEl: table,
        tableRect: toLocal(table.getBoundingClientRect()),
        colIndex,
        rowIndex,
        cellRect: toLocal((headerCell ?? cell).getBoundingClientRect()),
        rowRect: toLocal(row.getBoundingClientRect()),
      });
    };

    wrapper.addEventListener('mousemove', handleMouseMove);
    return () => {
      wrapper.removeEventListener('mousemove', handleMouseMove);
      if (clearTimerRef.current) clearTimeout(clearTimerRef.current);
    };
  }, [editor, getWrapper, toLocal]);

  // Track active cell + table rect from cursor position
  const [cursorTableRect, setCursorTableRect] = useState<DOMRect | null>(null);
  const cursorTableElRef = useRef<HTMLTableElement | null>(null);
  const cursorInTable = useRef(false);

  // State update for React-dependent rendering (menus, dots, select button)
  useEffect(() => {
    const update = () => {
      if (!editor.isActive('table')) {
        setActiveCellRect(null);
        setCursorTableRect(null);
        cursorTableElRef.current = null;
        cursorInTable.current = false;
        return;
      }
      cursorInTable.current = true;
      const { $from } = editor.state.selection;
      const cellNode = editor.view.domAtPos($from.pos);
      const cellEl = (cellNode.node instanceof HTMLElement ? cellNode.node : cellNode.node.parentElement)?.closest('td, th') as HTMLElement | null;
      const tableEl = cellEl?.closest('table') as HTMLTableElement | null;
      if (cellEl) {
        setActiveCellRect(toLocal(cellEl.getBoundingClientRect()));
      } else {
        setActiveCellRect(null);
      }
      if (tableEl) {
        setCursorTableRect(toLocal(tableEl.getBoundingClientRect()));
        cursorTableElRef.current = tableEl;
      } else {
        setCursorTableRect(null);
        cursorTableElRef.current = null;
      }
    };
    // ResizeObserver for instant bar repositioning on table size changes
    let resizeObserver: ResizeObserver | null = null;
    const observeTable = () => {
      resizeObserver?.disconnect();
      if (cursorTableElRef.current) {
        resizeObserver = new ResizeObserver(() => updateBarPositions());
        resizeObserver.observe(cursorTableElRef.current);
      }
    };

    const onSelection = () => { update(); observeTable(); };
    editor.on('selectionUpdate', onSelection);
    editor.on('transaction', update);
    return () => {
      editor.off('selectionUpdate', onSelection);
      editor.off('transaction', update);
      resizeObserver?.disconnect();
    };
  }, [editor, toLocal]);

  // Direct DOM updates for bar + highlight positions (called from events, not RAF)
  const updateBarPositions = useCallback(() => {
    const wrapper = getWrapper();
    const table = cursorTableElRef.current;
    if (!wrapper || !table || !cursorInTable.current) return;

    const wr = wrapper.getBoundingClientRect();
    const st = wrapper.scrollTop;
    const tr = table.getBoundingClientRect();
    const tLocal = { x: tr.left - wr.left, y: tr.top - wr.top + st, w: tr.width, h: tr.height };

    if (colBarRef.current) {
      colBarRef.current.style.left = `${tLocal.x + tLocal.w}px`;
      colBarRef.current.style.top = `${tLocal.y}px`;
      colBarRef.current.style.height = `${tLocal.h}px`;
    }
    if (rowBarRef.current) {
      rowBarRef.current.style.left = `${tLocal.x}px`;
      rowBarRef.current.style.top = `${tLocal.y + tLocal.h}px`;
      rowBarRef.current.style.width = `${tLocal.w}px`;
    }

    try {
      const { $from } = editor.state.selection;
      const cn = editor.view.domAtPos($from.pos);
      const cel = (cn.node instanceof HTMLElement ? cn.node : cn.node.parentElement)?.closest('td, th') as HTMLElement | null;
      if (cel && cellHighlightRef.current) {
        const cr = cel.getBoundingClientRect();
        cellHighlightRef.current.style.left = `${cr.left - wr.left}px`;
        cellHighlightRef.current.style.top = `${cr.top - wr.top + st}px`;
        cellHighlightRef.current.style.width = `${cr.width}px`;
        cellHighlightRef.current.style.height = `${cr.height}px`;
        cellHighlightRef.current.style.display = '';
      }
    } catch {
      if (cellHighlightRef.current) cellHighlightRef.current.style.display = 'none';
    }
  }, [editor, getWrapper]);

  // Update bar positions when state changes trigger re-renders
  useEffect(() => {
    updateBarPositions();
  });

  // Close menu on click outside
  useEffect(() => {
    if (!menu) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenu(null);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [menu]);

  // Deselect table on click outside
  useEffect(() => {
    if (!tableSelected) return;
    const handleClick = (e: MouseEvent) => {
      if (controlsRef.current?.contains(e.target as Node)) return;
      if (hover?.tableEl?.contains(e.target as Node)) return;
      setTableSelected(false);
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [tableSelected, hover]);

  const focusCellAt = useCallback((table: HTMLTableElement, rowIdx: number, colIdx: number) => {
    const rows = table.querySelectorAll('tr');
    const cell = rows[rowIdx]?.children[colIdx];
    if (cell) {
      try {
        const pos = editor.view.posAtDOM(cell, 0);
        editor.chain().focus().setTextSelection(pos).run();
      } catch {
        editor.chain().focus().run();
      }
    }
  }, [editor]);

  const runAndClose = useCallback((fn: () => void) => {
    // Ensure editor has focus (cursor should already be in the right cell from menu open)
    editor.chain().focus().run();
    fn();
    setMenu(null);
    setHover(null);
    setTableSelected(false);
  }, [editor]);

  // Find the table node position and info from cursor position or hover
  const getTableInfo = useCallback(() => {
    const { state } = editor;

    // Try cursor position first (works after focusCellAt)
    const { $from } = state.selection;
    for (let d = $from.depth; d > 0; d--) {
      if ($from.node(d).type.name === 'table') {
        return { pos: $from.before(d), node: $from.node(d) };
      }
    }

    // Fallback: find by hover DOM element
    if (!hover) return null;
    const { doc } = state;
    let tablePos = -1;
    let tableNode: any = null;

    doc.descendants((node, pos) => {
      if (tableNode) return false;
      if (node.type.name === 'table') {
        const dom = editor.view.nodeDOM(pos);
        if (dom === hover.tableEl) {
          tablePos = pos;
          tableNode = node;
          return false;
        }
      }
      return true;
    });

    return tablePos >= 0 ? { pos: tablePos, node: tableNode } : null;
  }, [editor, hover]);

  const duplicateRow = useCallback(() => {
    const info = getTableInfo();
    if (!info || !hover) return;
    const { pos: tablePos, node: tableNode } = info;
    const row = tableNode.child(hover.rowIndex);
    // Position after the current row
    let rowEndPos = tablePos + 1; // skip table open tag
    for (let i = 0; i <= hover.rowIndex; i++) {
      rowEndPos += tableNode.child(i).nodeSize;
    }
    const { tr } = editor.state;
    tr.insert(rowEndPos, row.copy(row.content));
    editor.view.dispatch(tr);
  }, [editor, hover, getTableInfo]);

  const duplicateColumn = useCallback(() => {
    const info = getTableInfo();
    if (!info || !hover) return;
    const { pos: tablePos, node: tableNode } = info;
    const { tr } = editor.state;

    // Walk rows in reverse so inserts don't shift positions of earlier rows
    for (let r = tableNode.childCount - 1; r >= 0; r--) {
      const row = tableNode.child(r);
      if (hover.colIndex >= row.childCount) continue;
      const cell = row.child(hover.colIndex);
      // Find position after this cell
      let cellEndPos = tablePos + 1; // table open
      for (let ri = 0; ri < r; ri++) cellEndPos += tableNode.child(ri).nodeSize;
      cellEndPos += 1; // row open
      for (let ci = 0; ci <= hover.colIndex; ci++) cellEndPos += row.child(ci).nodeSize;
      tr.insert(cellEndPos, cell.copy(cell.content));
    }

    editor.view.dispatch(tr);
  }, [editor, hover, getTableInfo]);

  const clearRowContent = useCallback(() => {
    const info = getTableInfo();
    if (!info || !hover) return;
    const { pos: tablePos, node: tableNode } = info;
    const { tr } = editor.state;
    const row = tableNode.child(hover.rowIndex);

    let rowStartPos = tablePos + 1;
    for (let i = 0; i < hover.rowIndex; i++) rowStartPos += tableNode.child(i).nodeSize;
    rowStartPos += 1; // row open tag

    // Clear each cell's content (replace with empty paragraph)
    const emptyPara = editor.state.schema.nodes.paragraph.create();
    for (let c = row.childCount - 1; c >= 0; c--) {
      const cell = row.child(c);
      let cellPos = rowStartPos;
      for (let ci = 0; ci < c; ci++) cellPos += row.child(ci).nodeSize;
      const contentStart = cellPos + 1; // cell open tag
      const contentEnd = cellPos + cell.nodeSize - 1; // before cell close tag
      tr.replaceWith(contentStart, contentEnd, emptyPara);
    }

    editor.view.dispatch(tr);
  }, [editor, hover, getTableInfo]);

  const clearColumnContent = useCallback(() => {
    const info = getTableInfo();
    if (!info || !hover) return;
    const { pos: tablePos, node: tableNode } = info;
    const { tr } = editor.state;
    const emptyPara = editor.state.schema.nodes.paragraph.create();

    // Process rows in reverse to keep positions stable
    for (let r = tableNode.childCount - 1; r >= 0; r--) {
      const row = tableNode.child(r);
      if (hover.colIndex >= row.childCount) continue;
      const cell = row.child(hover.colIndex);

      let rowStartPos = tablePos + 1;
      for (let ri = 0; ri < r; ri++) rowStartPos += tableNode.child(ri).nodeSize;
      rowStartPos += 1; // row open

      let cellPos = rowStartPos;
      for (let ci = 0; ci < hover.colIndex; ci++) cellPos += row.child(ci).nodeSize;

      const contentStart = cellPos + 1;
      const contentEnd = cellPos + cell.nodeSize - 1;
      tr.replaceWith(contentStart, contentEnd, emptyPara);
    }

    editor.view.dispatch(tr);
  }, [editor, hover, getTableInfo]);

  const clearTableContent = useCallback(() => {
    const info = getTableInfo();
    if (!info) return;
    const { pos: tablePos, node: tableNode } = info;
    const { tr } = editor.state;
    const emptyPara = editor.state.schema.nodes.paragraph.create();

    // Clear all cells in reverse order
    for (let r = tableNode.childCount - 1; r >= 0; r--) {
      const row = tableNode.child(r);
      let rowStartPos = tablePos + 1;
      for (let ri = 0; ri < r; ri++) rowStartPos += tableNode.child(ri).nodeSize;
      rowStartPos += 1; // row open

      for (let c = row.childCount - 1; c >= 0; c--) {
        const cell = row.child(c);
        let cellPos = rowStartPos;
        for (let ci = 0; ci < c; ci++) cellPos += row.child(ci).nodeSize;
        const contentStart = cellPos + 1;
        const contentEnd = cellPos + cell.nodeSize - 1;
        tr.replaceWith(contentStart, contentEnd, emptyPara);
      }
    }

    editor.view.dispatch(tr);
  }, [editor, getTableInfo]);

  const openColumnMenu = useCallback(() => {
    if (!hover) return;
    focusCellAt(hover.tableEl, 0, hover.colIndex);
    setMenu({
      type: 'column',
      x: hover.cellRect.x + hover.cellRect.width / 2,
      y: hover.cellRect.y,
    });
  }, [hover, focusCellAt]);

  const openRowMenu = useCallback(() => {
    if (!hover) return;
    focusCellAt(hover.tableEl, hover.rowIndex, 0);
    setMenu({
      type: 'row',
      x: hover.rowRect.x,
      y: hover.rowRect.y + hover.rowRect.height / 2,
    });
  }, [hover, focusCellAt]);

  // Store tableRect when selected so it persists after mouse leaves
  const lastTableRectRef = useRef<DOMRect | null>(null);
  if (hover) lastTableRectRef.current = hover.tableRect;

  const tableRect = hover?.tableRect ?? cursorTableRect ?? lastTableRectRef.current;
  if (!tableRect) return null;

  const showPlusButtons = !tableSelected && (mouseOverTable || !!activeCellRect);

  return (
    <div ref={controlsRef}>
      {/* Active cell highlight — positioned by RAF loop */}
      <div
        ref={cellHighlightRef}
        className="absolute z-10 pointer-events-none border-2 border-primary rounded-sm"
        style={{ display: activeCellRect ? '' : 'none' }}
      />

      {/* Small select button at top-left — only on hover */}
      {!tableSelected && hover && mouseOverTable && (
        <button
          className="absolute z-30 flex h-5 w-5 cursor-pointer items-center justify-center rounded bg-muted text-muted-foreground shadow-sm hover:bg-muted-foreground/20 transition-colors"
          style={{
            left: tableRect.x - 10,
            top: tableRect.y - 10,
          }}
          title="Select table"
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => setTableSelected(true)}
        >
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.5">
            <rect x="1" y="1" width="8" height="8" rx="1" />
            <line x1="5" y1="1" x2="5" y2="9" />
            <line x1="1" y1="5" x2="9" y2="5" />
          </svg>
        </button>
      )}

      {/* Blue border + full-width bar — only when table is selected */}
      {tableSelected && (
        <>
          <div
            className="absolute z-20 pointer-events-none rounded border-2 border-primary"
            style={{
              left: tableRect.x - 3,
              top: tableRect.y - 3,
              width: tableRect.width + 6,
              height: tableRect.height + 6,
            }}
          />
          <button
            className="absolute z-30 flex h-6 cursor-pointer items-center justify-center rounded-t-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
            style={{
              left: tableRect.x - 3,
              top: tableRect.y - 27,
              width: tableRect.width + 6,
            }}
            title="Table options"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => {
              setMenu({
                type: 'table',
                x: tableRect.x + tableRect.width / 2,
                y: tableRect.y - 4,
              });
            }}
          >
            <MoreHorizontalIcon className="h-4 w-4" />
          </button>
        </>
      )}

      {/* Add column bar (right) + Add row bar (bottom) — visible on hover or when typing */}
      {showPlusButtons && (
        <>
        {/* Add column bar on the right — positioned by RAF loop */}
        <button
          ref={colBarRef}
          className="absolute z-30 flex w-5 cursor-pointer items-center justify-center rounded-r-md border border-l-0 bg-muted/60 text-muted-foreground/60 hover:bg-primary/10 hover:text-primary transition-colors"
          title="Add column"
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => {
            const table = hover?.tableEl ?? cursorTableElRef.current;
            if (table) focusCellAt(table, 0, (table.querySelector('tr')?.children.length ?? 1) - 1);
            editor.chain().focus().addColumnAfter().run();
          }}
        >
          <PlusSignIcon className="h-3 w-3" />
        </button>

        {/* Add row bar at the bottom — positioned by RAF loop */}
        <button
          ref={rowBarRef}
          className="absolute z-30 flex h-5 cursor-pointer items-center justify-center rounded-b-md border border-t-0 bg-muted/60 text-muted-foreground/60 hover:bg-primary/10 hover:text-primary transition-colors"
          title="Add row"
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => {
            const table = hover?.tableEl ?? cursorTableElRef.current;
            if (table) focusCellAt(table, table.querySelectorAll('tr').length - 1, 0);
            editor.chain().focus().addRowAfter().run();
          }}
        >
          <PlusSignIcon className="h-3 w-3" />
        </button>
        </>
      )}

      {/* Column ⋯ above hovered column — only on mouse hover */}
      {!menu && !tableSelected && hover && mouseOverTable && (
        <button
          className="absolute z-30 flex h-5 w-8 cursor-pointer items-center justify-center rounded-md bg-muted text-muted-foreground shadow-sm hover:bg-muted-foreground/20 transition-colors"
          style={{
            left: hover.cellRect.x + hover.cellRect.width / 2 - 16,
            top: hover.cellRect.y - 10,
          }}
          onMouseDown={(e) => e.preventDefault()}
          onClick={openColumnMenu}
          title="Column options"
        >
          <MoreHorizontalIcon className="h-3.5 w-3.5" />
        </button>
      )}

      {/* Row ⋮ left of hovered row — only on mouse hover */}
      {!menu && !tableSelected && hover && mouseOverTable && (
        <button
          className="absolute z-30 flex h-8 w-5 cursor-pointer items-center justify-center rounded-md bg-muted text-muted-foreground shadow-sm hover:bg-muted-foreground/20 transition-colors"
          style={{
            left: hover.rowRect.x - 10,
            top: hover.rowRect.y + hover.rowRect.height / 2 - 16,
          }}
          onMouseDown={(e) => e.preventDefault()}
          onClick={openRowMenu}
          title="Row options"
        >
          <MoreVerticalIcon className="h-3.5 w-3.5" />
        </button>
      )}

      {/* Context menu */}
      {menu && (
        <div
          ref={menuRef}
          className="absolute z-50 w-48 rounded-lg border bg-popover p-1 shadow-md"
          style={{
            left: menu.type === 'table' ? menu.x - 96 : menu.type === 'column' ? menu.x - 96 : menu.x - 200,
            top: menu.type === 'table' ? menu.y : menu.type === 'column' ? menu.y - 8 : menu.y - 16,
          }}
        >
          {menu.type === 'table' ? (
            <>
              <MenuItem icon={EraserIcon} label="Clear content" onClick={() => runAndClose(() => clearTableContent())} />
              <MenuItem icon={Delete01Icon} label="Delete table" destructive onClick={() => runAndClose(() => editor.chain().focus().deleteTable().run())} />
            </>
          ) : menu.type === 'column' ? (
            <>
              <MenuItem icon={ArrowLeft02Icon} label="Add column left" onClick={() => runAndClose(() => editor.chain().focus().addColumnBefore().run())} />
              <MenuItem icon={ArrowRight02Icon} label="Add column right" onClick={() => runAndClose(() => editor.chain().focus().addColumnAfter().run())} />
              <MenuItem icon={Copy01Icon} label="Duplicate" onClick={() => runAndClose(() => duplicateColumn())} />
              <div className="my-1 h-px bg-border" />
              <MenuItem icon={EraserIcon} label="Clear content" onClick={() => runAndClose(() => clearColumnContent())} />
              <MenuItem icon={Delete01Icon} label="Delete" destructive onClick={() => runAndClose(() => editor.chain().focus().deleteColumn().run())} />
            </>
          ) : (
            <>
              <ToggleItem
                label="Header row"
                checked={hover?.tableEl?.querySelector('tr:first-child th') !== null}
                onChange={() => {
                  focusCellAt(hover!.tableEl, 0, 0);
                  editor.chain().focus().toggleHeaderRow().run();
                  setMenu(null);
                }}
              />
              <div className="my-1 h-px bg-border" />
              <MenuItem icon={ArrowUp02Icon} label="Add row above" onClick={() => runAndClose(() => editor.chain().focus().addRowBefore().run())} />
              <MenuItem icon={ArrowDown02Icon} label="Add row below" onClick={() => runAndClose(() => editor.chain().focus().addRowAfter().run())} />
              <MenuItem icon={Copy01Icon} label="Duplicate" onClick={() => runAndClose(() => duplicateRow())} />
              <div className="my-1 h-px bg-border" />
              <MenuItem icon={EraserIcon} label="Clear content" onClick={() => runAndClose(() => clearRowContent())} />
              <MenuItem icon={Delete01Icon} label="Delete" destructive onClick={() => runAndClose(() => editor.chain().focus().deleteRow().run())} />
            </>
          )}
          {menu.type !== 'table' && (
            <>
              <div className="my-1 h-px bg-border" />
              <MenuItem icon={CancelCircleIcon} label="Delete table" destructive onClick={() => runAndClose(() => editor.chain().focus().deleteTable().run())} />
            </>
          )}
        </div>
      )}
    </div>
  );
}

function MenuItem({ icon: Icon, label, onClick, destructive }: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  onClick: () => void;
  destructive?: boolean;
}) {
  return (
    <button
      className={`flex w-full cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-accent ${
        destructive ? 'text-destructive hover:text-destructive' : ''
      }`}
      onMouseDown={(e) => e.preventDefault()}
      onClick={onClick}
    >
      <Icon className="h-4 w-4 shrink-0" />
      {label}
    </button>
  );
}

function ToggleItem({ label, checked, onChange }: {
  label: string;
  checked: boolean;
  onChange: () => void;
}) {
  return (
    <button
      className="flex w-full cursor-pointer items-center justify-between rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-accent"
      onMouseDown={(e) => e.preventDefault()}
      onClick={onChange}
    >
      <span>{label}</span>
      <span
        className={`relative inline-flex h-4 w-7 items-center rounded-full transition-colors ${
          checked ? 'bg-primary' : 'bg-muted-foreground/30'
        }`}
      >
        <span
          className={`inline-block h-3 w-3 rounded-full bg-white transition-transform ${
            checked ? 'translate-x-3.5' : 'translate-x-0.5'
          }`}
        />
      </span>
    </button>
  );
}
