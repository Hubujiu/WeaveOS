import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Link, Navigate, Route, Routes, useNavigate, useSearchParams } from 'react-router';
import brandLogin from './assets/waveos-brand-login.svg';
import brandRegister from './assets/waveos-brand-register.svg';
import glowLogin from './assets/waveos-glow-login.svg';
import glowRegister from './assets/waveos-glow-register.svg';
import errorIcon from './assets/waveos-error.svg';
import userLoginIcon from './assets/waveos-user-login.svg';
import userRegisterIcon from './assets/waveos-user-register.svg';
import lockIcon from './assets/waveos-lock-login.svg';
import eyeIcon from './assets/waveos-eye-login.svg';
import invitationIcon from './assets/waveos-invitation.svg';
import checkDisabled from './assets/waveos-check-disabled.svg';
import googleDisabled from './assets/waveos-google-disabled.svg';
import microsoftDisabled from './assets/waveos-microsoft-disabled.svg';
import githubDisabled from './assets/waveos-github-disabled.svg';

type User = { id: string; account: string };
const passwordClasses = [/[A-Z]/, /[a-z]/, /[0-9]/, /[\x21-\x2f\x3a-\x40\x5b-\x60\x7b-\x7e]/];
type Envelope<T> = { code: string; message: string; data: T; meta: { requestId: string } | null };

class ApiError extends Error {
  constructor(message: string, readonly status: number) { super(message); }
}

async function api<T>(path: string, method = 'GET', body?: object): Promise<T> {
  let response: Response;
  const headers: Record<string, string> = body ? { 'Content-Type': 'application/json' } : {};
  if (!['GET', 'HEAD'].includes(method)) {
    const csrf = document.cookie.split(';').map(value => value.trim()).find(value => value.startsWith('__Host-csrf='));
    if (csrf) headers['X-CSRF-Token'] = csrf.slice('__Host-csrf='.length);
  }
  try {
    response = await fetch(`/api/v1/${path}`, {
      method,
      credentials: 'include',
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError('网络连接失败，请检查网络后重试', 0);
  }
  if (!response.ok) {
    if (response.status === 401) throw new ApiError('账号或密码错误', 401);
    if (response.status === 409) throw new ApiError('账号已存在', 409);
    if (response.status === 400 || response.status === 403) throw new ApiError('输入信息或邀请码无效', response.status);
    throw new ApiError('服务暂时不可用，请稍后重试', response.status);
  }
  if (response.status === 204) return undefined as T;
  const envelope = await response.json() as Envelope<T>;
  if (envelope.code !== 'OK' || !envelope.data) throw new Error('服务响应无效');
  return envelope.data;
}

function BrandHeader({ register = false }: { register?: boolean }) {
  return <header className="brand-header"><div className="brand-identity"><img src={register ? brandRegister : brandLogin} width="44" height="32" alt="" /><span>WaveOS</span></div><span className="brand-slogan">安全 · 高效 · 连接世界</span></header>;
}

function AuthLayout({ children, register = false }: { children: ReactNode; register?: boolean }) {
  return <div className="site"><BrandHeader register={register} /><main className="auth-main"><img className="auth-glow" src={register ? glowRegister : glowLogin} width="1260" height="1260" alt="" /><div className="auth-card">{children}</div></main></div>;
}

function AccountField({ value, onChange, register = false, credentialError = false }: { value: string; onChange: (value: string) => void; register?: boolean; credentialError?: boolean }) {
  return <div className="auth-field"><label htmlFor="account">邮箱 / 用户名{register && <span className="required"> *</span>}</label><div className={`field-input${credentialError ? ' credential-error' : ''}`}><img src={register ? userRegisterIcon : userLoginIcon} width="24" height="24" alt="" /><input id="account" name="account" aria-label="账号" type="text" value={value} onChange={event => onChange(event.target.value)} placeholder={register ? '请输入邮箱或用户名' : 'user@example.com'} autoComplete="username" required /></div></div>;
}

function PasswordField({ id = 'password', label = '密码', value, onChange, showStrength = false, register = false }: { id?: string; label?: string; value: string; onChange: (value: string) => void; showStrength?: boolean; register?: boolean }) {
  const [visible, setVisible] = useState(false);
  const strength = passwordClasses.filter(pattern => pattern.test(value)).length;
  return <div className="auth-field"><label htmlFor={id}>{label}{register && <span className="required"> *</span>}</label>
    <div className="field-input password-field"><img src={lockIcon} width="24" height="24" alt="" />
      <input id={id} name={id} aria-label={label} type={visible ? 'text' : 'password'} value={value} onChange={event => onChange(event.target.value)} placeholder={id === 'confirmation' ? '请再次输入密码' : '请输入密码'} autoComplete={id === 'confirmation' || register ? 'new-password' : 'current-password'} required />
      <button type="button" className="visibility" aria-label={visible ? '隐藏密码' : '显示密码'} onClick={() => setVisible(!visible)}><img src={eyeIcon} width="24" height="24" alt="" /></button>
    </div>
    {showStrength && <div className="strength" role="progressbar" aria-label="密码强度" aria-valuemin={0} aria-valuemax={4} aria-valuenow={strength}>{[1, 2, 3, 4].map(segment => <span key={segment} className={segment <= strength ? 'active' : ''} />)}</div>}
  </div>;
}

function accountError(account: string): string | null {
  if (!account) return '请输入账号';
  if (account.includes(' ')) return '账号不能包含空格';
  if (Array.from(account).length > 254) return '账号过长';
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
    if (!password) { setError('请输入密码'); return; }
    setPending(true);
    setError('');
    try {
      await api<User>('sessions', 'POST', { account: account, password });
      navigate('/app', { replace: true });
    } catch (cause) { setError(cause instanceof ApiError && cause.status === 401 ? '邮箱或密码不正确' : cause instanceof Error ? cause.message : '登录失败'); }
    finally { setPending(false); }
  }

  return <AuthLayout><form className="auth-form login-form" onSubmit={submit} noValidate>
    <div className="form-heading"><h1>登录</h1><p className="subtitle">登录您的账号，开始高效沟通</p></div>
    {error && <div role="alert" className="form-error"><img src={errorIcon} width="26" height="26" alt="" /><span><strong>{error}</strong><small>{error === '邮箱或密码不正确' ? '请检查您的密码后重新输入。为方便您，邮箱地址已保留。' : '请检查输入后重试。'}</small></span></div>}
    <AccountField value={account} onChange={setAccount} credentialError={error === '邮箱或密码不正确'} />
    <PasswordField value={password} onChange={setPassword} />
    <div className="login-options"><label className="remember-disabled"><input type="checkbox" aria-label="记住账号" disabled defaultChecked /><img src={checkDisabled} width="22" height="22" alt="" /><span>记住账号</span></label><button className="forgot-disabled" type="button" disabled>忘记密码?</button></div>
    <button className="primary-button" type="submit" disabled={pending}>{pending ? '登录中…' : '登录'}</button>
    <div className="social-divider"><span />或使用第三方账号登录<span /></div>
    <div className="social-buttons">{[['Google', googleDisabled], ['Microsoft', microsoftDisabled], ['GitHub', githubDisabled]].map(([name, icon]) => <button key={name} type="button" disabled><img src={icon} width="24" height="24" alt="" /><span>{name}</span></button>)}</div>
    <p className="form-prompt">还没有账号？ <Link className="secondary-link" to="/register">立即注册</Link></p>
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
    if (/[^\x20-\x7e]/.test(password)) { setError('密码仅允许 ASCII 可打印字符'); return; }
    if (!passwordClasses.every(pattern => pattern.test(password))) { setError('密码需要大写字母、小写字母、数字和特殊符号'); return; }
    if (password !== confirmation) { setError('两次输入的密码不一致'); return; }
    if (!invitationCode.trim()) { setError('请输入邀请码'); return; }
    setPending(true);
    setError('');
    try {
      await api<User>('registrations', 'POST', { account: account, password, invitationCode: invitationCode.trim() });
      navigate('/login', { replace: true });
    } catch (cause) { setError(cause instanceof Error ? cause.message : '注册失败'); }
    finally { setPending(false); }
  }

  return <AuthLayout register><form className="auth-form register-form" onSubmit={submit} noValidate>
    <div className="form-heading"><h1>注册</h1><p className="subtitle">创建您的账号，开始使用 WaveOS</p></div>
    <AccountField value={account} onChange={setAccount} register />
    <PasswordField value={password} onChange={setPassword} showStrength register />
    <PasswordField id="confirmation" label="确认密码" value={confirmation} onChange={setConfirmation} register />
    <div className="auth-field"><label htmlFor="invitationCode">邀请码<span className="required"> *</span></label><div className="field-input"><img src={invitationIcon} width="24" height="24" alt="" /><input id="invitationCode" name="invitationCode" aria-label="邀请码" type="text" value={invitationCode} onChange={event => setInvitationCode(event.target.value)} placeholder="请输入邀请码" required /></div></div>
    {error && <p role="alert" className="form-error">{error}</p>}
    <button className="primary-button" type="submit" disabled={pending}>{pending ? '注册中…' : '注册'}</button>
    <p className="form-prompt">已有账号？ <Link className="secondary-link" to="/login">立即登录</Link></p>
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
    api<User>('sessions/current').then(value => { if (active) { setUser(value); setChecking(false); } }).catch(cause => {
      if (!active) return;
      if (cause instanceof ApiError && cause.status === 401) {
        navigate('/login', { replace: true });
      } else {
        setError(cause instanceof Error ? cause.message : '服务暂时不可用，请稍后重试');
        setChecking(false);
      }
    });
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
  if (!user) return <div className="site"><BrandHeader /><main className="app-main"><h1>无法验证登录态</h1><p role="alert">{error}</p><button className="primary-button" type="button" onClick={() => window.location.reload()}>重试</button></main></div>;
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
