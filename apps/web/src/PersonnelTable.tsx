import { useLayoutEffect, useRef, type ReactNode } from 'react';
import './personnel-table.css';

export type PersonnelColumn<T> = {
  label: string;
  width?: number;
  render: (row: T) => ReactNode;
};

type Props<T extends { id: string }> = {
  label: string;
  rows: T[];
  columns: PersonnelColumn<T>[];
  minWidth: number;
  empty: string;
  selected?: (row: T) => boolean;
  selection?: {
    ids: string[];
    onChange: (ids: string[]) => void;
    label: (row: T) => string;
  };
};

// Q34: page-local Arca presentation. The caller owns queries, paging and edits.
export function PersonnelTable<T extends { id: string }>({ label, rows, columns, minWidth, empty, selected, selection }: Props<T>) {
  const selectAll = useRef<HTMLInputElement>(null);
  const all = rows.length > 0 && rows.every(row => selection?.ids.includes(row.id));
  const some = rows.some(row => selection?.ids.includes(row.id));
  useLayoutEffect(() => {
    if (selectAll.current) selectAll.current.indeterminate = some && !all;
  }, [some, all]);

  return <div className="table-scroll" tabIndex={0} aria-label={label + '表格滚动区域'}>
    <table className="personnel-data-table" aria-label={label} style={{ minWidth }}>
      <colgroup>
        {selection && <col style={{ width: 48 }} />}
        {columns.map(column => <col key={column.label} style={column.width ? { width: column.width } : undefined} />)}
      </colgroup>
      <thead><tr>
        {selection && <th scope="col" className="member-selector"><input ref={selectAll} type="checkbox" aria-label="选择当前页成员" aria-checked={some && !all ? 'mixed' : all} disabled={!rows.length} checked={all} onChange={e => selection.onChange(e.target.checked ? rows.map(row => row.id) : [])} /></th>}
        {columns.map(column => <th scope="col" key={column.label}>{column.label}</th>)}
      </tr></thead>
      <tbody>
        {rows.map((row, index) => {
          const checked = selection?.ids.includes(row.id) ?? selected?.(row);
          return <tr key={row.id} aria-selected={checked}>
            {selection && <td className="member-selector">
              <span className="table-row-number" aria-hidden="true">{index + 1}</span>
              <input type="checkbox" aria-label={selection.label(row)} checked={!!checked} onChange={() => selection.onChange(checked ? selection.ids.filter(id => id !== row.id) : [...selection.ids, row.id])} />
            </td>}
            {columns.map(column => {
              const value = column.render(row);
              return <td key={column.label}><div className="table-cell" title={typeof value === 'string' || typeof value === 'number' ? String(value) : undefined}>{value}</div></td>;
            })}
          </tr>;
        })}
        {!rows.length && <tr><td colSpan={columns.length + (selection ? 1 : 0)} className="empty-state">{empty}</td></tr>}
      </tbody>
    </table>
  </div>;
}
