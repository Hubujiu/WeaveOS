import type { ReactNode } from "react";

export type SortDirection = "asc" | "desc";

export type SortState = {
  key: string;
  direction: SortDirection;
};

export type TableColumn<T> = {
  /** Stable key; also the default object property read for the cell + sort value. */
  key: string;
  /** Header content. */
  header: ReactNode;
  /** Allow clicking the header to sort by this column. Defaults to true. */
  sortable?: boolean;
  /** Q36 typed columns: only number/time may expose sorting. Untyped legacy
   * columns retain their old behavior until the business adapter is migrated. */
  dataType?: 'text' | 'number' | 'time' | 'enum' | 'boolean' | 'action';
  /** Cell text alignment. */
  align?: "left" | "center" | "right";
  /** Column width as a CSS length, e.g. "160px" or "20%". Omit to share remaining space equally. */
  width?: string;
  /** Custom cell renderer. Falls back to `row[key]`. */
  cell?: (row: T) => ReactNode;
  /** Render an inline text input for this column's cells (ignored when `cell` is set). */
  editable?: boolean;
  /** Value used for sorting. Falls back to `row[key]`. */
  sortValue?: (row: T) => string | number;
};

export type InsertPosition = "before" | "after";

export interface TableProps<T> {
  /** WeaveOS: received page and authoritative server total; never slice twice. */
  manualPagination?: boolean;
  recordCount?: number;
  /** WeaveOS: fit existing flex viewport and add inert visual grid rows. */
  fillViewport?: boolean;
  minWidth?: number;
  ariaLabel?: string;
  selectAllLabel?: string;
  getRowLabel?: (row: T) => string;
  data: T[];
  columns: TableColumn<T>[];
  /** Controlled display IDs; all columns still retain their order and widths. */
  hiddenColumnIds?: readonly string[];
  /** Stable id per row, required for correct selection across sorts. Defaults to row index. */
  getRowId?: (row: T, index: number) => string;
  /** Render a leading index column; row numbers become checkboxes on hover. */
  selectable?: boolean;
  selectedRowIds?: string[];
  defaultSelectedRowIds?: string[];
  onSelectionChange?: (ids: string[]) => void;
  sort?: SortState | null;
  defaultSort?: SortState | null;
  onSortChange?: (sort: SortState | null) => void;
  /** Sorting is performed on the complete server result. Never reorder a page. */
  manualSorting?: boolean;
  /** Allow dragging the right edge of a header to resize that column. */
  resizable?: boolean;
  /** Minimum column width in px when resizing. */
  minColumnWidth?: number;
  onColumnResize?: (key: string, width: number) => void;
  columnWidths?: Record<string, number>;
  /** All affected widths, including neighboring columns, on resize commit. */
  onColumnWidthsChange?: (widths: Record<string, number>) => void;
  /** Allow dragging a header grip to reorder columns. */
  reorderable?: boolean;
  onColumnOrderChange?: (keys: string[]) => void;
  columnOrder?: string[];
  /** Called when an `editable` cell changes. */
  onCellEdit?: (rowId: string, columnKey: string, value: string) => void;
  /** When set, non-sortable headers become editable inputs for the column name. */
  onColumnRename?: (columnKey: string, value: string) => void;
  /** Enables the row menu (Insert before / after). Receives the target index. */
  onInsertRow?: (index: number, position: InsertPosition) => void;
  /** Enables Delete in the row menu. */
  onDeleteRow?: (rowId: string, index: number) => void;
  /** Enables the column menu (Insert before / after). Receives the target column index. */
  onInsertColumn?: (index: number, position: InsertPosition) => void;
  /** Enables Delete in the column menu. */
  onDeleteColumn?: (columnKey: string, index: number) => void;
  /** Fixed row height in px — required for virtualization. */
  rowHeight?: number;
  /** Scroll viewport height in px. */
  height?: number;
  /** Rows rendered above/below the viewport. */
  overscan?: number;
  /** Client-side page controls under the table (ReUI Data Grid layout). */
  paginated?: boolean;
  pageSize?: number;
  defaultPageSize?: number;
  onPageSizeChange?: (size: number) => void;
  /** Public Q36 page API is 1-based. Server pages are not silently clamped. */
  page?: number;
  defaultPage?: number;
  onPageChange?: (page: number) => void;
  /** @deprecated Transitional 0-based API for the existing PersonnelTable. */
  pageIndex?: number;
  defaultPageIndex?: number;
  onPageIndexChange?: (index: number) => void;
  /** Page-size options in the footer select. */
  pageSizes?: number[];
  /** Fires when the viewport scrolls near the bottom — load the next page. */
  onEndReached?: () => void;
  /** Currently fetching — shows skeleton rows and pauses `onEndReached`. */
  loading?: boolean;
  /** How many skeleton rows to show while loading more (default 3). */
  skeletonRows?: number;
  emptyState?: ReactNode;
  className?: string;
}

/** A data row paired with its stable id. */
export type TableRow<T> = { row: T; id: string };

/** Ref map from column key to its header cell, shared across the resize/reorder hooks. */
export type HeaderCellRefs = {
  current: Record<string, HTMLTableCellElement | null>;
};
