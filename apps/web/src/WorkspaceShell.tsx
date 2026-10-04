import type { ReactNode } from 'react';
import type { Access, User } from './workspace-types';
import homeIcon from './applications/assets/sidebar-home.svg';
import catalogIcon from './applications/assets/sidebar-catalog.svg';
import settingsIcon from './applications/assets/settings.svg';
import './monochrome.css';

type WorkspaceShellProps = {
 user: User; access: Access; catalog: boolean; appId?: string; admin: boolean;
 accountOpen: boolean; onAccount: () => void; onHome: () => void;
 onCatalog: () => void; onAdmin: () => void; tabs: ReactNode; children: ReactNode;
};

/** Persistent layout only: identity, navigation guards and data remain owned by Workspace. */
export function WorkspaceShell({ user, access, catalog, appId, admin, accountOpen, onAccount, onHome, onCatalog, onAdmin, tabs, children }: WorkspaceShellProps) {
 const home = !catalog && !appId && !admin;
 return <div className="mono-shell app-shell" data-testid="workspace-shell">
  <aside className="mono-rail" data-testid="workspace-rail" aria-label="工作区快捷入口">
   <span className="mono-mark" aria-hidden="true">w.</span>
   <button type="button" className="mono-rail-button" data-testid="workspace-nav-home" aria-label="返回主页" aria-current={home ? 'page' : undefined} onClick={onHome}><img src={homeIcon} alt="" /></button>
   <button type="button" className="mono-rail-button" aria-label="应用中心" aria-current={catalog ? 'page' : undefined} onClick={onCatalog}><img src={catalogIcon} alt="" /></button>
   <div className="mono-rail-spacer" />
   {access.personnelManage && <button type="button" className="mono-rail-button" data-testid="workspace-nav-admin" aria-label="设置" aria-current={admin ? 'page' : undefined} onClick={onAdmin}><img src={settingsIcon} alt="" /></button>}
   <button type="button" className="mono-account" aria-label="账号" aria-expanded={accountOpen} onClick={onAccount}>{user.account.slice(0, 2)}</button>
  </aside>
  <div className="mono-surface" data-testid="workspace-surface">
   <nav className="mono-sidebar" aria-label="应用导航">
    <span className="mono-brand">WaveOS</span>
    <p className="mono-caption">我的工作区</p>
    <button type="button" className="mono-nav-item" aria-current={home ? 'page' : undefined} onClick={onHome}><img src={homeIcon} alt="" /><span>首页</span></button>
    <p className="mono-caption mono-section-caption">应用</p>
    <button type="button" className="mono-nav-item" aria-current={catalog ? 'page' : undefined} onClick={onCatalog}><img src={catalogIcon} alt="" /><span>全部应用</span></button>
    {access.personnelManage && <button type="button" className="mono-nav-item" aria-current={admin ? 'page' : undefined} onClick={onAdmin}><img src={settingsIcon} alt="" /><span>人员管理</span></button>}
   </nav>
   <div className="mono-main" data-testid="application-shell">
    <header className="mono-top">{tabs}</header>
    <div className="mono-content-inset"><div className="mono-content-well" data-testid="workspace-content-well">{children}</div></div>
   </div>
  </div>
 </div>;
}
