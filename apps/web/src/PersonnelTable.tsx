import { useMemo, type ReactNode } from 'react';
import { Table, type TableColumn } from './vendor/arca/components/motion/table';
import './vendor/arca/arca.css';
import './personnel-table.css';

export type PersonnelColumn<T> = {
  label: string;
  width?: number;
  render: (row: T) => ReactNode;
  sortValue?: (row: T) => string | number;
  sortable?: boolean;
};
type Props<T extends { id: string }> = {
  label: string;
  rows: T[];
  columns: PersonnelColumn<T>[];
  minWidth: number;
  empty: string;
  pagination: { page: number; pageSize: number; total: number; onPageChange: (page: number) => void; onPageSizeChange: (size: number) => void };
  selection?: { ids: string[]; onChange: (ids: string[]) => void; label: (row: T) => string };
};

// Q35: actual fixed upstream Table. This bridge only owns business props;
// original rendering, motion, sorting, drag, resize, scrollbar and pager remain upstream.
export function PersonnelTable<T extends { id: string }>({ label, rows, columns, minWidth, empty, pagination, selection }: Props<T>) {
  const sourceColumns = useMemo<TableColumn<T>[]>(() => columns.map(column => ({
    key: column.label,
    header: column.label,
    width: column.width ? `${column.width}px` : undefined,
    align: 'left',
    sortable: column.sortable,
    sortValue: column.sortValue ?? (row => { const value = column.render(row); return typeof value === 'string' || typeof value === 'number' ? value : ''; }),
    cell: row => { const value = column.render(row); return <div className="table-cell" title={typeof value === 'string' || typeof value === 'number' ? String(value) : undefined}>{value}</div>; },
  })), [columns]);
  return <div className="arca-source personnel-source-scope"><Table
    className="personnel-source-table rounded-xl"
    ariaLabel={label}
    data={rows}
    columns={sourceColumns}
    getRowId={row => row.id}
    minWidth={minWidth}
    rowHeight={40}
    fillViewport
    paginated
    manualPagination
    recordCount={pagination.total}
    pageIndex={pagination.page - 1}
    pageSize={pagination.pageSize}
    pageSizes={[5, 10, 20, 25, 50, 100]}
    onPageIndexChange={index => pagination.onPageChange(index + 1)}
    onPageSizeChange={pagination.onPageSizeChange}
    resizable
    reorderable
    selectable={!!selection}
    selectedRowIds={selection?.ids}
    onSelectionChange={selection?.onChange}
    selectAllLabel="选择当前页成员"
    getRowLabel={selection?.label}
    emptyState={empty}
  /></div>;
}
