// Isolated component evidence only. No backend requests or business adapter.
import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Table } from './vendor/arca/components/motion/table';
import { QueryFilterPanel } from './QueryFilterPanel';
import type { EventFilterGroup, MemberFilterGroup } from './query-contracts';
import './vendor/arca/arca.css';
import './q36-front-fixture.css';

const rows = [
  { id: 'first', account: 'Zulu', occurredAt: '2026-10-01T08:00:00Z', amount: 8 },
  { id: 'second', account: 'Alice', occurredAt: '2026-10-01T09:00:00Z', amount: 3 },
];
const columns = [
  { key: 'account', header: '成员', dataType: 'text' as const, sortable: true, width: '260px' },
  { key: 'occurredAt', header: '发生时间', dataType: 'time' as const, width: '300px' },
  { key: 'amount', header: '数字示例', dataType: 'number' as const, width: '220px' },
];
function Fixture() {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [selection, setSelection] = useState<string[]>([]);
  const [order, setOrder] = useState(['account', 'occurredAt', 'amount']);
  const [widths, setWidths] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(false);
  const [empty, setEmpty] = useState(false);
  const [members, setMembers] = useState<MemberFilterGroup>();
  const [events, setEvents] = useState<EventFilterGroup>();
  const [applied, setApplied] = useState('未提交');
  const [eventView, setEventView] = useState(false);
  const [sort, setSort] = useState<{key: string; direction: 'asc' | 'desc'} | null>(null);
  const controlled = { page, onPageChange: setPage, columnOrder: order, columnWidths: widths,
    onColumnWidthsChange: setWidths, manualSorting: true };
  return <main className="q36-fixture arca-source">
    <header><h1>人员管理</h1><p>Q36 B1 · 合成组件场景，无后端验收</p></header>
    <div className="q36-fixture-controls">
      <button onClick={() => setLoading(x => !x)}>切换加载</button>
      <button onClick={() => setEmpty(x => !x)}>切换空表</button>
      <button onClick={() => { setOrder(['amount', 'account', 'occurredAt']); setWidths({amount: 150, account: 330, occurredAt: 300}); }}>外部恢复列状态</button>
      <button onClick={() => setEventView(x => !x)}>切换记录筛选</button>
      <button onClick={() => {setMembers({operator:'and',children:[{operator:'or',children:[]}]} as unknown as MemberFilterGroup);}}>载入空子组</button>
      <button onClick={() => {setMembers({operator:'and',children:Array.from({length:20},()=>({field:'account' as const,operator:'eq' as const,value:'Alice'}))} as MemberFilterGroup);}}>载入20条件</button>
    </div>
    <section className="q36-fixture-surface">
      <div className="q36-fixture-toolbar">
        <label className="q36-fixture-search">搜索成员<input aria-label="搜索成员" placeholder="搜索姓名或账号" /></label>
        {eventView ? <QueryFilterPanel view="events" value={events} onApply={filter => {setEvents(filter);setApplied(JSON.stringify(filter) ?? '无筛选');}} />
          : <QueryFilterPanel view="members" value={members} options={{identityIds:[{value:'00000000-0000-4000-8000-000000000001',label:'普通员工'}],departmentIds:[{value:'00000000-0000-4000-8000-000000000002',label:'研发部'}]}} onApply={filter => {setMembers(filter);setApplied(JSON.stringify(filter) ?? '无筛选');}} />}
      </div>
      <Table {...controlled} data={empty ? [] : rows} columns={columns} getRowId={r=>r.id}
        selectable selectedRowIds={selection} onSelectionChange={setSelection}
        onColumnOrderChange={setOrder} resizable reorderable fillViewport paginated manualPagination
        pageSize={pageSize} onPageSizeChange={setPageSize} recordCount={empty ? 0 : 420} rowHeight={40}
        loading={loading} sort={sort} onSortChange={setSort} ariaLabel="受控成员" getRowLabel={r=>'选择 '+r.account} />
    </section>
    <output aria-label="当前页">{page}</output><output aria-label="选择结果">{selection.join(',')}</output>
    <output aria-label="排序请求">{JSON.stringify(sort)}</output><output aria-label="筛选提交">{applied}</output>
  </main>;
}
createRoot(document.getElementById('root')!).render(<Fixture />);
