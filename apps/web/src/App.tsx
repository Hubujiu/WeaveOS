import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Link, Navigate, Route, Routes, useNavigate, useSearchParams } from 'react-router';
import brand from './assets/brand.svg';

type User = { id: string; account: string };
type Envelope<T> = { code: string; message: string; data: T; meta: { requestId: string } | null };

async function api<T>(path: string, method = 'GET', body?: object): Promise<T> {
  const response = await fetch(`/api/v1/${path}`, {
    method,
    credentials: 'include',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    if (response.status === 401) throw new Error('账号或密码错误');
    if (response.status === 409) throw new Error('账号已存在');
    if (response.status === 400 || response.status === 403) throw new Error('输入信息或邀请码无效');
    throw new Error('服务暂时不可用，请稍后重试');
  }
  if (response.status === 204) return undefined as T;
  const envelope = await response.json() as Envelope<T>;
  if (envelope.code !== 'OK' || !envelope.data) throw new Error('服务响应无效');
  return envelope.data;
}

function BrandHeader() {
  return <header className="brand-header"><img src={brand} width="20" height="24" alt="" /><span>DocWeave</span></header>;
}

function AuthLayout({ children }: { children: ReactNode }) {
  return <div className="site"><BrandHeader /><main className="auth-main"><div className="auth-card">{children}</div></main></div>;
}

function AccountField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return <><label htmlFor="account">账号</label><input id="account" name="account" type="text" value={value} onChange={event => onChange(event.target.value)} placeholder="输入账号" autoComplete="username" maxLength={254} required /></>;
}

function PasswordField({ id = 'password', label = '密码', value, onChange, showStrength = false }: { id?: string; label?: string; value: string; onChange: (value: string) => void; showStrength?: boolean }) {
  const [visible, setVisible] = useState(false);
  const strength = [/[A-Z]/, /[a-z]/, /[0-9]/, /[^A-Za-z0-9]/].filter(pattern => pattern.test(value)).length;
  return <>
    <label htmlFor={id}>{label}</label>
    <div className="password-field">
      <input id={id} name={id} type={visible ? 'text' : 'password'} value={value} onChange={event => onChange(event.target.value)} placeholder={id === 'confirmation' ? '再次输入密码' : '输入密码'} autoComplete={id === 'confirmation' ? 'new-password' : 'current-password'} required />
      <button type="button" className="visibility" aria-label={visible ? '隐藏密码' : '显示密码'} onClick={() => setVisible(!visible)}>{visible ? '隐藏' : '显示'}</button>
      {showStrength && <div className="strength" role="progressbar" aria-label="密码强度" aria-valuemin={0} aria-valuemax={4} aria-valuenow={strength}>{[1, 2, 3, 4].map(segment => <span key={segment} className={segment <= strength ? 'active' : ''} />)}</div>}
    </div>
  </>;
}

function accountError(account: string): string | null {
  if (!account.trim()) return '请输入账号';
  if (account.includes(' ')) return '账号不能包含空格';
  if (account.trim().length > 254) return '账号过长';
  return null;
}

function Login() {
  const navigate = useNavigate();
  const [account, setAccount] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (pending) return;
    const invalid = accountError(account);
    if (invalid) { setError(invalid); return; }
    setPending(true);
    setError('');
    try {
      await api<User>('sessions', 'POST', { account: account.trim(), password });
      navigate('/app', { replace: true });
    } catch (cause) { setError(cause instanceof Error ? cause.message : '登录失败'); }
    finally { setPending(false); }
  }

  return <AuthLayout><form onSubmit={submit} noValidate>
    <h1>登录</h1><p className="subtitle">使用企业账号登录</p>
    <AccountField value={account} onChange={setAccount} />
    <PasswordField value={password} onChange={setPassword} />
    {error && <p role="alert" className="form-error">{error}</p>}
    <button className="primary-button" type="submit" disabled={pending}>{pending ? '登录中…' : '登录'}</button>
    <Link className="secondary-link" to="/register">没有账号？注册</Link>
  </form></AuthLayout>;
}

function Register() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [account, setAccount] = useState('');
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [invitationCode, setInvitationCode] = useState(() => searchParams.get('invitationCode') ?? '');
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (pending) return;
    const invalid = accountError(account);
    if (invalid) { setError(invalid); return; }
    if (!/[A-Z]/.test(password) || !/[a-z]/.test(password) || !/[0-9]/.test(password) || !/[^A-Za-z0-9]/.test(password)) { setError('密码需要大写字母、小写字母、数字和特殊符号'); return; }
    if (password !== confirmation) { setError('两次输入的密码不一致'); return; }
    if (!invitationCode.trim()) { setError('请输入邀请码'); return; }
    setPending(true);
    setError('');
    try {
      await api<User>('registrations', 'POST', { account: account.trim(), password, invitationCode: invitationCode.trim() });
      navigate('/login', { replace: true });
    } catch (cause) { setError(cause instanceof Error ? cause.message : '注册失败'); }
    finally { setPending(false); }
  }

  return <AuthLayout><form onSubmit={submit} noValidate>
    <h1>注册</h1><p className="subtitle">创建新账号后即可登录。</p>
    <AccountField value={account} onChange={setAccount} />
    <PasswordField value={password} onChange={setPassword} showStrength />
    <PasswordField id="confirmation" label="确认密码" value={confirmation} onChange={setConfirmation} />
    <label htmlFor="invitationCode">邀请码</label><input id="invitationCode" name="invitationCode" type="text" value={invitationCode} onChange={event => setInvitationCode(event.target.value)} placeholder="输入邀请码" required />
    {error && <p role="alert" className="form-error">{error}</p>}
    <button className="primary-button" type="submit" disabled={pending}>{pending ? '注册中…' : '注册'}</button>
    <Link className="secondary-link" to="/login">已有账号？登录</Link>
  </form></AuthLayout>;
}

function ProtectedApp() {
  const navigate = useNavigate();
  const [user, setUser] = useState<User | null>(null);
  const [error, setError] = useState('');
  const [checking, setChecking] = useState(true);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    let active = true;
    api<User>('sessions/current').then(value => { if (active) { setUser(value); setChecking(false); } }).catch(() => { if (active) navigate('/login', { replace: true }); });
    return () => { active = false; };
  }, [navigate]);

  async function logout() {
    if (pending) return;
    setPending(true);
    setError('');
    try { await api<void>('sessions/current', 'DELETE'); navigate('/login', { replace: true }); }
    catch (cause) { setError(cause instanceof Error ? cause.message : '退出失败'); }
    finally { setPending(false); }
  }

  if (checking) return <div className="site"><BrandHeader /><main className="app-main" aria-live="polite">正在验证登录态…</main></div>;
  return <div className="site"><BrandHeader /><main className="app-main"><h1>欢迎</h1><p>{user?.account}</p>{error && <p role="alert">{error}</p>}<button className="primary-button" type="button" onClick={logout} disabled={pending}>退出登录</button></main></div>;
}

export function App() {
  return <Routes>
    <Route path="/login" element={<Login />} />
    <Route path="/register" element={<Register />} />
    <Route path="/app" element={<ProtectedApp />} />
    <Route path="*" element={<Navigate to="/app" replace />} />
  </Routes>;
}
