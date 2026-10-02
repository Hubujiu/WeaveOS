"use client";
// beui.dev/components/motion/table

import { useVirtualizer } from "@tanstack/react-virtual";
import { useReducedMotion } from "motion/react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Checkbox } from "../checkbox";
import { cn } from "../../../lib/utils";
import { EditableCell } from "./editable-cell";
import { RowHandle } from "./row-handle";
import { SkeletonRows } from "./skeleton-rows";
import { TableHeader } from "./table-header";
import { TablePagination } from "./table-pagination";
import { TableScrollbar } from "./table-scrollbar";
import type { HeaderCellRefs, SortDirection, TableProps } from "./types";
import { useColumnReorder } from "./use-column-reorder";
import { useColumnResize } from "./use-column-resize";
import { useColumnSort } from "./use-column-sort";
import { useRowSelection } from "./use-row-selection";
import { alignText, INDEX_PX, INDEX_WIDTH, readCell } from "./utils";

export type {
  SortDirection,
  SortState,
  TableColumn,
  TableProps,
} from "./types";

/** The root font size Tailwind's rem scale assumes, and the pre-measure guess. */
const DEFAULT_ROOT_FONT_SIZE = 16;

/**
 * What one `rem` is worth here, in px. The default until the first client
 * layout, so the server and the hydrating client emit the same floor; measured
 * once after that, because a document that sets its own `html { font-size }`
 * lays a rem column out against that size and a floor computed from 16 would
 * fall short by the same factor.
 */
function useRootFontSize() {
  const [size, setSize] = useState(DEFAULT_ROOT_FONT_SIZE);
  useEffect(() => {
    const measured = Number.parseFloat(
      getComputedStyle(document.documentElement).fontSize,
    );
    if (measured > 0) setSize(measured);
  }, []);
  return size;
}

/**
 * The absolute width a column declared, in px, or null when it declared a share
 * of the remainder instead (`fr`, `%`, `auto`, `calc()`, nothing at all) — those
 * are worth whatever is left over, which is not a width this can add up.
 */
function resolveColumnWidth(
  width: string | undefined,
  rootFontSize: number,
): number | null {
  if (!width) return null;
  const value = Number.parseFloat(width);
  if (!Number.isFinite(value)) return null;
  if (width.endsWith("px")) return value;
  // rem is the other absolute length the repo writes.
  if (width.endsWith("rem")) return value * rootFontSize;
  return null;
}

export function Table<T>({
  data,
  columns,
  hiddenColumnIds = [],
  getRowId,
  selectable = false,
  selectedRowIds,
  defaultSelectedRowIds,
  onSelectionChange,
  sort: sortProp,
  defaultSort = null,
  onSortChange,
  manualSorting = false,
  resizable = false,
  minColumnWidth = 64,
  onColumnResize,
  columnWidths,
  onColumnWidthsChange,
  reorderable = false,
  onColumnOrderChange,
  columnOrder,
  onCellEdit,
  onColumnRename,
  onInsertRow,
  onDeleteRow,
  onInsertColumn,
  onDeleteColumn,
  rowHeight = 48,
  height = 440,
  overscan = 10,
  paginated = false,
  pageSize: pageSizeProp,
  defaultPageSize = 10,
  onPageSizeChange,
  page: pageProp,
  defaultPage = 1,
  onPageChange,
  pageIndex: pageIndexProp,
  defaultPageIndex = 0,
  onPageIndexChange,
  pageSizes = [5, 10, 20, 25, 50, 100],
  onEndReached,
  loading = false,
  skeletonRows = 3,
  emptyState = "No data",
  className,
  manualPagination = false,
  recordCount: serverCount,
  fillViewport = false,
  minWidth,
  ariaLabel,
  selectAllLabel,
  getRowLabel,
}: TableProps<T>) {
  const reduce = useReducedMotion();
  const scrollRef = useRef<HTMLDivElement>(null);
  const thRefs: HeaderCellRefs = useRef<
    Record<string, HTMLTableCellElement | null>
  >({});

  const rows = useMemo(
    () =>
      data.map((row, index) => ({
        row,
        id: getRowId ? getRowId(row, index) : String(index),
      })),
    [data, getRowId],
  );

  const {
    orderedColumns,
    dragKey,
    dropIndex,
    startReorder,
    moveReorder,
    endReorder,
  } = useColumnReorder({ columns, thRefs, onColumnOrderChange, columnOrder });

  const visibleColumns = orderedColumns.filter(column => !hiddenColumnIds.includes(column.key));

  const { sort, sortedRows, setSort } = useColumnSort({
    rows,
    columns,
    sort: sortProp,
    defaultSort,
    onSortChange,
    manualSorting,
  });

  const [internalPageIndex, setInternalPageIndex] = useState(onPageChange ? defaultPage - 1 : defaultPageIndex);
  const [internalPageSize, setInternalPageSize] = useState(defaultPageSize);
  const pageIndex = pageProp !== undefined ? pageProp - 1 : pageIndexProp ?? internalPageIndex;
  const pageSize = pageSizeProp ?? internalPageSize;

  const recordCount = serverCount ?? sortedRows.length;
  const pageCount = Math.max(1, Math.ceil(recordCount / pageSize) || 1);
  const currentPage = pageProp !== undefined && manualPagination ? Math.max(0,pageIndex) : Math.min(pageIndex, pageCount - 1);
  const pagedRows = paginated && !manualPagination
    ? sortedRows.slice(currentPage * pageSize, currentPage * pageSize + pageSize)
    : sortedRows;
  const visibleRows = pagedRows;
  const [viewportHeight, setViewportHeight] = useState(0);
  useEffect(() => {
    if (!fillViewport || !scrollRef.current) return;
    const el = scrollRef.current;
    const observer = new ResizeObserver(() => setViewportHeight(el.clientHeight));
    setViewportHeight(el.clientHeight);
    observer.observe(el);
    return () => observer.disconnect();
  }, [fillViewport]);

  const setPageIndex = useCallback(
    (next: number) => {
      const clamped = Math.max(0, Math.min(next, pageCount - 1));
      if (pageIndexProp === undefined && pageProp === undefined) setInternalPageIndex(clamped);
      onPageIndexChange?.(clamped);
      onPageChange?.(clamped + 1);
    },
    [onPageIndexChange, onPageChange, pageCount, pageIndexProp, pageProp],
  );

  const setPageSize = useCallback(
    (next: number) => {
      if (pageSizeProp === undefined) setInternalPageSize(next);
      onPageSizeChange?.(next);
      if (pageIndexProp === undefined && pageProp === undefined) setInternalPageIndex(0);
      onPageIndexChange?.(0);
      onPageChange?.(1);
    },
    [onPageIndexChange, onPageChange, onPageSizeChange, pageIndexProp, pageProp, pageSizeProp],
  );

  const handleSetSort = useCallback(
    (key: string, direction: SortDirection) => {
      setSort(key, direction);
      if (!manualPagination) setPageIndex(0);
    },
    [setPageIndex, setSort, manualPagination],
  );

  useEffect(() => {
    // An authoritative total can shrink between requests. Recover the actual
    // server page as well as the visible pager; clamping display alone strands it.
    if (pageProp === undefined && manualPagination && pageIndex !== currentPage) setPageIndex(currentPage);
  }, [manualPagination, pageProp, pageIndex, currentPage, setPageIndex]);

  const { widths, startResize, moveResize, endResize } = useColumnResize({
    orderedColumns: visibleColumns,
    thRefs,
    minColumnWidth,
    onColumnResize,
    columnWidths,
    onColumnWidthsChange,
  });

  const { selected, allSelected, someSelected, toggleAll, toggleRow } =
    useRowSelection({
      sortedRows: visibleRows,
      selectedRowIds,
      defaultSelectedRowIds,
      onSelectionChange,
    });

  const virtualizer = useVirtualizer({
    count: visibleRows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => rowHeight,
    overscan,
  });

  const virtualItems = virtualizer.getVirtualItems();
  const totalSize = virtualizer.getTotalSize();
  const paddingTop = virtualItems.length > 0 ? virtualItems[0].start : 0;
  const paddingBottom =
    virtualItems.length > 0
      ? totalSize - virtualItems[virtualItems.length - 1].end
      : 0;

  const hasRowMenu = !!(onInsertRow || onDeleteRow);
  const hasColumnMenu = !!(onInsertColumn || onDeleteColumn);
  const rootFontSize = useRootFontSize();
  const resolvedWidths = visibleColumns.map(column => widths[column.key] ?? resolveColumnWidth(column.width, rootFontSize));
  // Once explicit widths are controlled, an HTML table must not distribute
  // leftover container space over them and silently alter the requested px.
  const controlledTableWidth = columnWidths && Object.keys(columnWidths).length && resolvedWidths.every(width => width !== null)
    ? resolvedWidths.reduce<number>((sum, width) => sum + (width ?? 0), selectable ? INDEX_PX : 0)
    : undefined;
  const rowStart = paginated ? currentPage * pageSize : 0;

  // Infinite scroll: fire onEndReached once per near-bottom dwell, paused while
  // loading; the guard resets when the load completes.
  const endReachedRef = useRef(false);
  useEffect(() => {
    if (!loading) endReachedRef.current = false;
  }, [loading]);
  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el || paginated || !onEndReached || loading || endReachedRef.current)
      return;
    if (el.scrollHeight - el.scrollTop - el.clientHeight < rowHeight * 4) {
      endReachedRef.current = true;
      onEndReached();
    }
  }, [onEndReached, loading, rowHeight, paginated]);
  const [activeColumn, setActiveColumn] = useState<string | null>(null);
  // Small delay on leave so the pointer can cross the gap from the header cell
  // to the portal handle without the column deactivating.
  const deactivateTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const activateColumn = useCallback((key: string) => {
    if (deactivateTimer.current) clearTimeout(deactivateTimer.current);
    deactivateTimer.current = null;
    setActiveColumn(key);
  }, []);
  const deactivateColumn = useCallback(() => {
    if (deactivateTimer.current) clearTimeout(deactivateTimer.current);
    deactivateTimer.current = setTimeout(() => setActiveColumn(null), 100);
  }, []);

  const rowRefs = useRef<Record<string, HTMLTableRowElement | null>>({});
  const [activeRow, setActiveRow] = useState<{ id: string; index: number } | null>(
    null,
  );
  const rowTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const activateRow = useCallback((id: string, index: number) => {
    if (rowTimer.current) clearTimeout(rowTimer.current);
    rowTimer.current = null;
    setActiveRow({ id, index });
  }, []);
  const deactivateRow = useCallback(() => {
    if (rowTimer.current) clearTimeout(rowTimer.current);
    rowTimer.current = setTimeout(() => setActiveRow(null), 100);
  }, []);
  const activeRowEl = activeRow ? rowRefs.current[activeRow.id] : null;
  const leadColumns = visibleColumns.length + (selectable ? 1 : 0);
  // Fillers are deliberately outside data, virtualizer, selection and paging.
  const remaining = fillViewport ? Math.max(0, viewportHeight - rowHeight - visibleRows.length * rowHeight) : 0;
  const blankRows = Array.from({ length: Math.ceil(remaining / rowHeight) }, (_, index) => Math.min(rowHeight, remaining - index * rowHeight));

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: 0 });
  }, [currentPage, pageSize]);

  return (
    <div
      className={cn(
        "flex w-full flex-col overflow-hidden border border-border bg-background text-sm",
        className,
      )}
    >
      <div className={cn("el-scrollbar relative w-full overflow-hidden", fillViewport && "min-h-0 flex-1")}>
        <div
          ref={scrollRef}
          tabIndex={0}
          aria-label={ariaLabel + '表格滚动区域'}
          onScroll={handleScroll}
          className="table-scroll el-scrollbar__wrap el-scrollbar__wrap--hidden-default overflow-x-auto overflow-y-auto"
          style={
            // Collapsed grid borders add half a pixel to the sticky header.
            // Round its occupied space up so keyboard-focused actions stay below it.
            fillViewport ? { height: '100%', maxHeight: 'none', scrollPaddingTop: rowHeight + 1 } : paginated
              ? { maxHeight: height }
              : { height }
          }
        >
          <div className="el-scrollbar__view">
            <table
              className="personnel-data-table w-full border-collapse"
              aria-label={ariaLabel}
              aria-busy={loading}
              style={{ tableLayout: "fixed", width: controlledTableWidth || "100%", minWidth: hiddenColumnIds.length && controlledTableWidth ? controlledTableWidth : minWidth }}
            >
          <colgroup>
            {selectable ? <col style={{ width: INDEX_WIDTH }} /> : null}
            {visibleColumns.map((column) => {
              const override = widths[column.key];
              const declared = resolveColumnWidth(column.width, rootFontSize);
              const width = override
                ? `${override}px`
                : declared
                  ? `${declared}px`
                  : undefined;
              return (
                <col key={column.key} style={width ? { width } : undefined} />
              );
            })}
          </colgroup>

          <TableHeader
            columns={visibleColumns}
            rowHeight={rowHeight}
            reduce={!!reduce}
            thRefs={thRefs}
            selectable={selectable}
            selectAllLabel={selectAllLabel}
            selectionDisabled={!visibleRows.length || loading}
            allSelected={allSelected}
            someSelected={someSelected}
            onToggleAll={toggleAll}
            sort={sort}
            onSetSort={handleSetSort}
            resizable={resizable}
            onResizeStart={startResize}
            onResizeMove={moveResize}
            onResizeEnd={endResize}
            reorderable={reorderable}
            dragKey={dragKey}
            dropIndex={dropIndex === null ? null : orderedColumns.slice(0,dropIndex).filter(column=>!hiddenColumnIds.includes(column.key)).length}
            onReorderStart={startReorder}
            onReorderMove={moveReorder}
            onReorderEnd={endReorder}
            onInsertColumn={onInsertColumn}
            onDeleteColumn={onDeleteColumn}
            onColumnRename={onColumnRename}
            activeColumn={hasColumnMenu ? activeColumn : null}
            onColumnActivate={hasColumnMenu ? activateColumn : undefined}
            onColumnDeactivate={hasColumnMenu ? deactivateColumn : undefined}
          />

          <tbody>
            {sortedRows.length === 0 && (!fillViewport || loading) ? (
              loading ? (
                <SkeletonRows
                  count={Math.max(1, Math.ceil((fillViewport ? viewportHeight - rowHeight : height) / rowHeight))}
                  columns={visibleColumns}
                  selectable={selectable}
                  rowHeight={rowHeight}
                />
              ) : (
                <tr>
                  <td
                    colSpan={leadColumns}
                    className="p-10 text-center text-muted-foreground"
                  >
                    {emptyState}
                  </td>
                </tr>
              )
            ) : (
              <>
                {paddingTop > 0 ? (
                  <tr aria-hidden style={{ height: paddingTop }}>
                    <td colSpan={leadColumns} />
                  </tr>
                ) : null}
                {virtualItems.map((vItem) => {
                  const entry = visibleRows[vItem.index];
                  if (!entry) return null;
                  const isSelected = selected.has(entry.id);
                  const rowNumber = rowStart + vItem.index + 1;
                  return (
                    <tr
                      key={entry.id}
                      data-row-id={entry.id}
                      aria-selected={selectable ? isSelected : undefined}
                      ref={(el) => {
                        rowRefs.current[entry.id] = el;
                      }}
                      data-selected={isSelected}
                      style={{ height: rowHeight }}
                      onPointerEnter={
                        hasRowMenu
                          ? () => activateRow(entry.id, vItem.index)
                          : undefined
                      }
                      onPointerLeave={hasRowMenu ? deactivateRow : undefined}
                      className={cn(
                        "group border-border/60 border-b transition-colors",
                        "data-[selected=true]:bg-primary/5",
                        "hover:bg-muted/50",
                      )}
                    >
                      {selectable ? (
                        <td className="relative border-border border-r p-0">
                          <span
                            className={cn(
                              "pointer-events-none absolute inset-0 flex items-center justify-center tabular-nums text-muted-foreground text-xs leading-none",
                              "group-hover:opacity-0 group-focus-within:opacity-0",
                              isSelected && "opacity-0",
                            )}
                          >
                            {rowNumber}
                          </span>
                          <div
                            className={cn(
                              "absolute inset-0 flex items-center justify-center opacity-0",
                              "group-hover:opacity-100 group-focus-within:opacity-100",
                              isSelected && "opacity-100",
                            )}
                          >
                            <Checkbox
                              checked={isSelected}
                              disabled={loading}
                              onCheckedChange={() => toggleRow(entry.id)}
                              aria-label={getRowLabel?.(entry.row) ?? `Select row ${rowNumber}`}
                              className="justify-center gap-0"
                            />
                          </div>
                        </td>
                      ) : null}
                      {visibleColumns.map((column, columnIndex) => (
                        <td
                          key={column.key}
                          className={cn(
                              "truncate px-4 text-foreground",
                            alignText(column.align),
                            columnIndex < orderedColumns.length - 1 &&
                              "border-border border-r",
                          )}
                        >
                          {!column.cell && column.editable ? (
                            <EditableCell
                              value={String(readCell(entry.row, column) ?? "")}
                              label={`${column.key} for row ${rowNumber}`}
                              onChange={(next) =>
                                onCellEdit?.(entry.id, column.key, next)
                              }
                            />
                          ) : (
                            readCell(entry.row, column)
                          )}
                        </td>
                      ))}
                    </tr>
                  );
                })}
                {paddingBottom > 0 ? (
                  <tr aria-hidden style={{ height: paddingBottom }}>
                    <td colSpan={leadColumns} />
                  </tr>
                ) : null}
                {loading && !paginated ? (
                  <SkeletonRows
                    count={skeletonRows}
                    columns={visibleColumns}
                    selectable={selectable}
                    rowHeight={rowHeight}
                  />
                ) : null}
              </>
            )}
            {(!loading || visibleRows.length > 0) && blankRows.map((blankHeight, index) => <tr key={'blank-' + index} data-empty-row aria-hidden="true" style={{ height: blankHeight }} className="border-border/60 border-b">
              {Array.from({ length: leadColumns }, (_, cell) => <td key={cell} className={cn("p-0", cell < leadColumns - 1 && "border-border border-r")} />)}
            </tr>)}
          </tbody>
        </table>
          </div>
        </div>
        <TableScrollbar scrollRef={scrollRef} headerHeight={rowHeight} />
        {fillViewport && !visibleRows.length && !loading ? <div role="status" className="pointer-events-none absolute inset-0 flex items-center justify-center text-muted-foreground text-sm">{emptyState}</div> : null}
      </div>
      {paginated ? (
        <TablePagination
          pageIndex={currentPage}
          pageSize={pageSize}
          pageCount={pageCount}
          recordCount={recordCount}
          sizes={pageSizes}
          onPageIndexChange={setPageIndex}
          onPageSizeChange={setPageSize}
          className="arca-pagination shrink-0"
          rowsPerPageLabel="每页条数"
          info="{from} - {to} / 共 {count} 项"
          previousPageLabel="上一页"
          nextPageLabel="下一页"
        />
      ) : null}
      {hasRowMenu && activeRow ? (
        <RowHandle
          rowEl={activeRowEl}
          id={activeRow.id}
          index={activeRow.index}
          onInsertRow={onInsertRow}
          onDeleteRow={onDeleteRow}
          onEnter={() => activateRow(activeRow.id, activeRow.index)}
          onLeave={deactivateRow}
        />
      ) : null}
    </div>
  );
}
