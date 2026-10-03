import { useState } from 'react';
import type { Application } from '../types';
import cardIcon from '../assets/app-card.svg';

export function ApplicationCards({ applications, open }: { applications: readonly Application[]; open: (app: Application) => void }) {
 return <div className="app-card-grid">{applications.map(app => <button type="button" className="app-tile" aria-label={app.name} key={app.id} onClick={() => open(app)}><span className="app-icon"><img src={cardIcon} alt="" /></span><span className="app-tile-name">{app.name}</span><span className="app-tile-description">业务应用</span></button>)}</div>;
}

export function ApplicationCatalog({ applications, loading, error, retry, canCreate, create, open }: {
 applications: readonly Application[]; loading: boolean; error: string; retry: () => void; canCreate: boolean; create: (trigger: HTMLButtonElement) => void; open: (app: Application) => void;
}) {
 const [search, setSearch] = useState('');
 const matches = applications.filter(app => app.name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()));
 return <>
  <div className="app-page-heading"><h1>应用中心</h1><p>打开业务应用或新建空白应用</p></div>
  <div className="app-page-actions">{canCreate && <button className="admin-button primary" onClick={e => create(e.currentTarget)}>新建应用</button>}</div>
  <div className="app-search-row"><input type="search" aria-label="搜索应用名称" placeholder="搜索应用名称" value={search} onChange={e => setSearch(e.target.value)} /><span className="admin-button app-filter-label">全部应用</span></div>
  {loading ? <section className="app-surface app-state" role="status">正在加载应用…</section>
   : error ? <section className="app-surface app-state"><p role="alert">{error}</p><button className="admin-button" onClick={retry}>重试</button></section>
   : <section className="app-surface app-catalog-section"><h2>全部应用</h2>{matches.length ? <ApplicationCards applications={matches} open={open} /> : <div className="app-state"><h3>{search.trim() ? '没有匹配的应用' : '暂无可用应用'}</h3><p>{search.trim() ? '尝试其他应用名称。' : '获得应用访问权限后，将在这里显示。'}</p></div>}</section>}
 </>;
}
