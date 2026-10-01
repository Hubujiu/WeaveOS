"use client";

import {
  ArrowLeft,
  ArrowRight,
  DotsSixVertical,
  DotsThree,
  Trash,
} from "@phosphor-icons/react";
import { motion } from "motion/react";
import { type PointerEvent as ReactPointerEvent, useEffect } from "react";
import { createPortal } from "react-dom";
import { Checkbox } from "../checkbox";
import { SPRING_PRESS } from "../../../lib/ease";
import { TOUCH_GESTURE_CLASS } from "../../../lib/touch";
import { cn } from "../../../lib/utils";
import { ColumnFilterMenu } from "./column-filter-menu";
import { TableMenu } from "./table-menu";
import type {
  HeaderCellRefs,
  InsertPosition,
  SortDirection,
  SortState,
  TableColumn,
} from "./types";
import { alignFlex, alignText, COLUMN_ACTIVE_SHADOW } from "./utils";

export interface TableHeaderProps<T> {
  columns: TableColumn<T>[];
  rowHeight: number;
  reduce: boolean;
  thRefs: HeaderCellRefs;
  selectable: boolean;
  allSelected: boolean;
  someSelected: boolean;
  onToggleAll: () => void;
  sort: SortState | null;
  onSetSort: (key: string, direction: SortDirection) => void;
  resizable: boolean;
  onResizeStart: (key: string, e: ReactPointerEvent) => void;
  onResizeMove: (e: ReactPointerEvent) => void;
  onResizeEnd: (e: ReactPointerEvent) => void;
  reorderable: boolean;
  dragKey: string | null;
  dropIndex: number | null;
  onReorderStart: (key: string, e: ReactPointerEvent) => void;
  onReorderMove: (e: ReactPointerEvent) => void;
  onReorderEnd: (e: ReactPointerEvent) => boolean;
  onInsertColumn?: (index: number, position: InsertPosition) => void;
  onDeleteColumn?: (columnKey: string, index: number) => void;
  onColumnRename?: (columnKey: string, value: string) => void;
  activeColumn: string | null;
  onColumnActivate?: (key: string) => void;
  onColumnDeactivate?: () => void;
}

/** Column insert / delete menu items shared by the header cell and the portal handle. */
function columnMenuItems<T>(
  column: TableColumn<T>,
  index: number,
  onInsertColumn?: (index: number, position: InsertPosition) => void,
  onDeleteColumn?: (columnKey: string, index: number) => void,
) {
  return [
    ...(onInsertColumn
      ? [
          {
            label: "Insert before",
            icon: <ArrowLeft />,
            onSelect: () => onInsertColumn(index, "before"),
          },
          {
            label: "Insert after",
            icon: <ArrowRight />,
            onSelect: () => onInsertColumn(index, "after"),
          },
        ]
      : []),
    ...(onDeleteColumn
      ? [
          {
            label: "Delete column",
            icon: <Trash />,
            destructive: true,
            onSelect: () => onDeleteColumn(column.key, index),
          },
        ]
      : []),
  ];
}

/** The ellipse handle, portaled so it can sit on the column's top border without
 * the scroll container clipping it. Straddles the border to bridge hover. */
function ColumnHandle<T>({
  column,
  index,
  thRefs,
  onInsertColumn,
  onDeleteColumn,
  onEnter,
  onLeave,
}: {
  column: TableColumn<T>;
  index: number;
  thRefs: HeaderCellRefs;
  onInsertColumn?: (index: number, position: InsertPosition) => void;
  onDeleteColumn?: (columnKey: string, index: number) => void;
  onEnter: () => void;
  onLeave: () => void;
}) {
  useEffect(() => {
    window.addEventListener("scroll", onLeave, true);
    return () => window.removeEventListener("scroll", onLeave, true);
  }, [onLeave]);

  const el = thRefs.current[column.key];
  if (!el || typeof document === "undefined") return null;
  const rect = el.getBoundingClientRect();

  return createPortal(
    <div
      style={{
        position: "fixed",
        top: rect.top,
        left: rect.left + rect.width / 2,
        transform: "translate(-50%, -50%)",
        zIndex: 40,
      }}
      onPointerEnter={onEnter}
      onPointerLeave={onLeave}
    >
      <TableMenu
        ariaLabel={`${column.key} column options`}
        triggerClassName="flex h-2 w-6 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-sm transition-colors hover:bg-primary/90"
        trigger={<DotsThree size={12} />}
        items={columnMenuItems(column, index, onInsertColumn, onDeleteColumn)}
      />
    </div>,
    document.body,
  );
}

export function TableHeader<T>({
  columns,
  rowHeight,
  reduce,
  thRefs,
  selectable,
  allSelected,
  someSelected,
  onToggleAll,
  sort,
  onSetSort,
  resizable,
  onResizeStart,
  onResizeMove,
  onResizeEnd,
  reorderable,
  dragKey,
  dropIndex,
  onReorderStart,
  onReorderMove,
  onReorderEnd,
  onInsertColumn,
  onDeleteColumn,
  onColumnRename,
  activeColumn,
  onColumnActivate,
  onColumnDeactivate,
  selectAllLabel = 'Select all rows',
  selectionDisabled = false,
}: TableHeaderProps<T> & { selectAllLabel?: string; selectionDisabled?: boolean }) {
  const hasColumnMenu = !!(onInsertColumn || onDeleteColumn);
  const activeIndex = columns.findIndex((c) => c.key === activeColumn);
  return (
    <>
      {hasColumnMenu && activeColumn && activeIndex >= 0 ? (
        <ColumnHandle
          column={columns[activeIndex]}
          index={activeIndex}
          thRefs={thRefs}
          onInsertColumn={onInsertColumn}
          onDeleteColumn={onDeleteColumn}
          onEnter={() => onColumnActivate?.(activeColumn)}
          onLeave={() => onColumnDeactivate?.()}
        />
      ) : null}
      <thead>
        <tr style={{ height: rowHeight }}>
          {selectable ? (
            <th className="sticky top-0 z-10 border-border border-b border-r bg-muted p-0">
              <div className="flex h-full items-center justify-center">
                <Checkbox
                  checked={allSelected}
                  indeterminate={!allSelected && someSelected}
                  onCheckedChange={onToggleAll}
                  aria-label={selectAllLabel}
                  disabled={selectionDisabled}
                  className="justify-center gap-0"
                />
              </div>
            </th>
          ) : null}
          {columns.map((column, index) => {
            const active = sort?.key === column.key;
            const isDragging = dragKey === column.key;
            const isActive = activeColumn === column.key;
            const canSort = column.sortable !== false;
            const isLast = index === columns.length - 1;
            return (
              <th
                key={column.key}
                ref={(el) => {
                  thRefs.current[column.key] = el;
                }}
                onPointerEnter={() => onColumnActivate?.(column.key)}
                onPointerLeave={() => onColumnDeactivate?.()}
                style={isActive ? { boxShadow: COLUMN_ACTIVE_SHADOW } : undefined}
                aria-sort={
                  active
                    ? sort?.direction === "asc"
                      ? "ascending"
                      : "descending"
                    : undefined
                }
                data-drop={dragKey ? dropIndex === index : undefined}
                data-dropend={
                  dragKey
                    ? dropIndex === columns.length && index === columns.length - 1
                    : undefined
                }
                className={cn(
                  "group relative sticky top-0 z-10 overflow-visible border-border border-b bg-muted p-0 font-medium text-muted-foreground",
                  !isLast && "border-r",
                  "data-[drop=true]:before:absolute data-[drop=true]:before:inset-y-0 data-[drop=true]:before:left-0 data-[drop=true]:before:w-0.5 data-[drop=true]:before:bg-primary",
                  "data-[dropend=true]:after:absolute data-[dropend=true]:after:inset-y-0 data-[dropend=true]:after:right-0 data-[dropend=true]:after:w-0.5 data-[dropend=true]:after:bg-primary",
                )}
              >
                <motion.div
                  className={cn(
                    "flex h-full items-center",
                    alignFlex(column.align),
                  )}
                  style={{ height: rowHeight }}
                  animate={
                    reduce
                      ? { opacity: isDragging ? 0.5 : 1 }
                      : {
                          scale: isDragging ? 1.04 : 1,
                          opacity: isDragging ? 0.5 : 1,
                        }
                  }
                  transition={SPRING_PRESS}
                >
                  {reorderable ? (
                    <button
                      type="button"
                      aria-label={`拖动以调整 ${column.key} 列顺序`}
                      onPointerDown={(e) => onReorderStart(column.key, e)}
                      onPointerMove={onReorderMove}
                      onPointerUp={(e) => {
                        e.stopPropagation();
                        onReorderEnd(e);
                      }}
                      onPointerCancel={onReorderEnd}
                      className={cn(
                        "flex h-full w-6 shrink-0 items-center justify-center text-muted-foreground/80",
                        isDragging ? "cursor-grabbing" : "cursor-grab",
                        TOUCH_GESTURE_CLASS,
                      )}
                    >
                      <DotsSixVertical size={14} />
                    </button>
                  ) : null}
                  {onColumnRename && !canSort ? (
                    <input
                      value={
                        typeof column.header === "string" ? column.header : ""
                      }
                      aria-label={`Rename ${column.key} column`}
                      size={1}
                      onPointerDown={(e) => e.stopPropagation()}
                      onChange={(e) =>
                        onColumnRename(column.key, e.target.value)
                      }
                      className={cn(
                        "min-w-0 flex-1 truncate appearance-none rounded-md border-0 bg-transparent px-4 font-medium text-muted-foreground outline-none transition-colors focus:bg-muted focus:text-foreground",
                        alignText(column.align),
                      )}
                    />
                  ) : canSort ? (
                    <ColumnFilterMenu
                      columnKey={column.key}
                      header={column.header}
                      sort={sort}
                      onSort={onSetSort}
                      reduce={reduce}
                    />
                  ) : (
                    <span
                      className={cn(
                        "flex h-full min-w-0 flex-1 select-none items-center px-3",
                        alignFlex(column.align),
                      )}
                    >
                      <span className="truncate">{column.header}</span>
                    </span>
                  )}
                </motion.div>
                {resizable ? (
                  <button
                    type="button"
                    data-resize-handle
                    aria-label={`Resize ${column.key} column`}
                    tabIndex={-1}
                    onPointerDown={(e) => {
                      e.stopPropagation();
                      onResizeStart(column.key, e);
                    }}
                    onPointerMove={onResizeMove}
                    onPointerUp={(e) => {
                      e.stopPropagation();
                      onResizeEnd(e);
                    }}
                    className={cn(
                      "absolute top-0 right-0 z-10 h-full w-1.5 cursor-col-resize touch-none bg-transparent",
                      TOUCH_GESTURE_CLASS,
                    )}
                  />
                ) : null}
              </th>
            );
          })}
        </tr>
      </thead>
    </>
  );
}
