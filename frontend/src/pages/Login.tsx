import type { Component } from 'solid-js';
import { createSignal, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { Activity, Eye, EyeOff, LockKeyhole, Moon, RadioTower, ShieldCheck, Sun } from 'lucide-solid';
import { useAuth } from '../lib/auth';
import { APP_VERSION } from '../lib/version';
import { useTheme } from '../lib/theme';

const Login: Component = () => {
  const [username, setUsername] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [showPassword, setShowPassword] = createSignal(false);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal('');
  const { login } = useAuth();
  const { isDark, toggleTheme } = useTheme();
  const navigate = useNavigate();

  const handleSubmit = async (event: SubmitEvent) => {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      await login(username().trim(), password());
      navigate('/', { replace: true });
    } catch (reason) {
      setError((reason as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <main class="min-h-screen grid lg:grid-cols-[1.15fr_.85fr] bg-base">
      <section class="hidden lg:flex flex-col justify-between p-12 border-r border-subtle bg-surface">
        <div class="brand-lockup">
          <div class="brand-mark" aria-hidden="true"><span /><span /><span /></div>
          <div><div class="brand-name">miniACS</div><div class="brand-subtitle">Independent CWMP control plane</div></div>
        </div>

        <div class="max-w-2xl">
          <p class="text-[11px] uppercase tracking-[.18em] text-sky-500 font-semibold mb-4">TR-069 infrastructure</p>
          <h1 class="text-4xl xl:text-5xl font-semibold tracking-[-.035em] text-primary leading-[1.08]">
            Operate every CPE from one deterministic control plane.
          </h1>
          <p class="mt-5 text-base text-secondary max-w-xl leading-relaxed">
            Device inventory, provisioning, firmware delivery, fault response, and vendor-neutral telemetry—without a GenieACS dependency.
          </p>

          <div class="grid grid-cols-3 gap-px mt-10 border border-subtle bg-subtle rounded-[5px] overflow-hidden">
            <div class="bg-surface p-5"><RadioTower size={20} class="text-sky-500 mb-5" /><strong class="block text-sm">CWMP native</strong><span class="text-[11px] text-muted">TR-098 + TR-181</span></div>
            <div class="bg-surface p-5"><Activity size={20} class="text-sky-500 mb-5" /><strong class="block text-sm">Live telemetry</strong><span class="text-[11px] text-muted">Operational metrics</span></div>
            <div class="bg-surface p-5"><ShieldCheck size={20} class="text-sky-500 mb-5" /><strong class="block text-sm">Audited access</strong><span class="text-[11px] text-muted">RBAC + event trail</span></div>
          </div>
        </div>

        <div class="flex items-center justify-between text-[10px] text-muted"><span>SkydashNET infrastructure software</span><span>v{APP_VERSION}</span></div>
      </section>

      <section class="min-h-screen flex flex-col">
        <div class="h-16 flex items-center justify-between px-5 lg:px-8 border-b border-subtle">
          <div class="brand-lockup lg:hidden">
            <div class="brand-mark" aria-hidden="true"><span /><span /><span /></div>
            <div><div class="brand-name">miniACS</div><div class="brand-subtitle">CWMP control plane</div></div>
          </div>
          <span class="hidden lg:block text-[10px] uppercase tracking-[.14em] text-muted">Secure operator access</span>
          <button class="icon-button" onClick={toggleTheme} aria-label="Toggle theme">{isDark() ? <Sun size={17} /> : <Moon size={17} />}</button>
        </div>

        <div class="flex-1 flex items-center justify-center px-5 py-10">
          <div class="w-full max-w-[390px]">
            <div class="w-10 h-10 grid place-items-center border border-sky-500/30 bg-sky-500/10 rounded-[4px] text-sky-500 mb-6"><LockKeyhole size={19} /></div>
            <h2 class="text-2xl font-semibold tracking-[-.025em] text-primary">Operator sign in</h2>
            <p class="text-sm text-muted mt-2">Use your miniACS control-plane credentials.</p>

            <Show when={error()}>
              <div role="alert" class="mt-6 px-3 py-2.5 border border-red-500/30 bg-red-500/8 rounded-[3px] text-xs text-red-400">{error()}</div>
            </Show>

            <form onSubmit={handleSubmit} class="mt-7 space-y-5">
              <div>
                <label for="username" class="block text-[11px] font-semibold uppercase tracking-[.06em] text-secondary mb-2">Username</label>
                <input id="username" class="input h-10" value={username()} onInput={(event) => setUsername(event.currentTarget.value)} autocomplete="username" required autofocus placeholder="operator" />
              </div>
              <div>
                <label for="password" class="block text-[11px] font-semibold uppercase tracking-[.06em] text-secondary mb-2">Password</label>
                <div class="relative">
                  <input id="password" class="input h-10 pr-10" type={showPassword() ? 'text' : 'password'} value={password()} onInput={(event) => setPassword(event.currentTarget.value)} autocomplete="current-password" required placeholder="Enter password" />
                  <button type="button" class="absolute right-1 top-1 w-8 h-8 grid place-items-center text-muted hover:text-primary" onClick={() => setShowPassword(!showPassword())} aria-label={showPassword() ? 'Hide password' : 'Show password'}>
                    {showPassword() ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>
              <button type="submit" class="btn btn-primary w-full h-10" disabled={loading()}>
                <Show when={loading()} fallback="Sign in to control plane"><span class="spinner !w-4 !h-4 !border-white/40 !border-t-white" /> Authenticating…</Show>
              </button>
            </form>

            <div class="mt-7 pt-5 border-t border-subtle flex gap-2.5 text-[11px] text-muted leading-relaxed">
              <ShieldCheck size={15} class="shrink-0 mt-0.5" />
              Sessions are stored only for this browser session. Repeated failures are rate-limited and written to the security audit log.
            </div>
          </div>
        </div>
      </section>
    </main>
  );
};

export default Login;
