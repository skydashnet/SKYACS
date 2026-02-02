import type { Component } from 'solid-js';
import { createSignal, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { useAuth } from '../lib/auth';
import { useTheme } from '../lib/theme';
import { AlertCircle, Loader2, Sun, Moon, ArrowRight, Network } from 'lucide-solid';

const Login: Component = () => {
  const { login, isAuthenticated } = useAuth();
  const { isDark, toggleTheme } = useTheme();
  const navigate = useNavigate();
  const [username, setUsername] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);

  if (isAuthenticated()) {
    navigate('/', { replace: true });
  }

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await login(username(), password());
      navigate('/', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login gagal');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div class="min-h-screen bg-base flex transition-theme relative overflow-hidden">
      {/* Background Pattern */}
      <div class="absolute inset-0 opacity-[0.03] pointer-events-none" style={{
        "background-image": `url("data:image/svg+xml,%3Csvg width='60' height='60' viewBox='0 0 60 60' xmlns='http://www.w3.org/2000/svg'%3E%3Cg fill='none' fill-rule='evenodd'%3E%3Cg fill='%23${isDark() ? 'ffffff' : '000000'}' fill-opacity='1'%3E%3Cpath d='M36 34v-4h-2v4h-4v2h4v4h2v-4h4v-2h-4zm0-30V0h-2v4h-4v2h4v4h2V6h4V4h-4zM6 34v-4H4v4H0v2h4v4h2v-4h4v-2H6zM6 4V0H4v4H0v2h4v4h2V6h4V4H6z'/%3E%3C/g%3E%3C/g%3E%3C/svg%3E")`,
      }} />

      {/* Theme Toggle - Fixed Position */}
      <button 
        onClick={toggleTheme}
        class="fixed top-6 right-6 z-50 p-3 rounded-xl bg-elevated border border-default hover:border-teal-500/50 transition-all duration-200 group"
        title={isDark() ? 'Switch to Light Mode' : 'Switch to Dark Mode'}
      >
        <div class="transition-transform duration-200 group-hover:rotate-12">
          {isDark() ? <Moon size={18} class="text-secondary" /> : <Sun size={18} class="text-secondary" />}
        </div>
      </button>

      {/* Left Side - Branding */}
      <div class="hidden lg:flex lg:w-1/2 xl:w-3/5 relative items-center justify-center p-12">
        <div class="relative z-10 max-w-lg">
          {/* Decorative Elements */}
          <div class="absolute -top-20 -left-20 w-40 h-40 bg-gradient-to-br from-teal-500/20 to-cyan-500/10 rounded-full blur-3xl" />
          <div class="absolute -bottom-10 -right-10 w-32 h-32 bg-gradient-to-br from-cyan-500/15 to-teal-500/10 rounded-full blur-2xl" />
          
          {/* Logo - Hexagonal Shape */}
          <div class="relative mb-8">
            <div class="w-24 h-24 relative">
              <svg viewBox="0 0 100 100" class="w-full h-full">
                <defs>
                  <linearGradient id="logoGrad" x1="0%" y1="0%" x2="100%" y2="100%">
                    <stop offset="0%" style="stop-color:#14b8a6" />
                    <stop offset="100%" style="stop-color:#06b6d4" />
                  </linearGradient>
                </defs>
                <polygon 
                  points="50,5 95,27.5 95,72.5 50,95 5,72.5 5,27.5" 
                  fill="url(#logoGrad)"
                  class="drop-shadow-lg"
                />
                <text x="50" y="58" text-anchor="middle" fill="white" font-size="32" font-weight="700" font-family="system-ui">M</text>
              </svg>
            </div>
          </div>

          {/* Title */}
          <h1 class="text-5xl font-black text-primary tracking-tight mb-3">
            mini<span class="text-transparent bg-clip-text bg-gradient-to-r from-teal-500 to-cyan-500">ACS</span>
          </h1>
          <p class="text-xl font-medium text-secondary mb-8">
            TR-069 Management System
          </p>

          {/* Feature List */}
          <div class="space-y-4">
            <div class="flex items-center gap-3 text-secondary">
              <div class="w-8 h-8 rounded-lg bg-teal-500/10 flex items-center justify-center">
                <Network size={16} class="text-teal-500" />
              </div>
              <span>CPE Remote Management</span>
            </div>
            <div class="flex items-center gap-3 text-secondary">
              <div class="w-8 h-8 rounded-lg bg-cyan-500/10 flex items-center justify-center">
                <svg class="w-4 h-4 text-cyan-500" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/>
                </svg>
              </div>
              <span>Firmware Updates & Provisioning</span>
            </div>
            <div class="flex items-center gap-3 text-secondary">
              <div class="w-8 h-8 rounded-lg bg-emerald-500/10 flex items-center justify-center">
                <svg class="w-4 h-4 text-emerald-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 0l-2 2a1 1 0 101.414 1.414L8 10.414l1.293 1.293a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
                </svg>
              </div>
              <span>Real-time Monitoring</span>
            </div>
          </div>

          {/* Version Badge */}
          <div class="mt-12 inline-flex items-center gap-2 px-4 py-2 rounded-full bg-elevated border border-default">
            <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
            <span class="text-xs font-medium text-muted">v1.0.0-beta</span>
            <span class="text-xs text-muted opacity-50">|</span>
            <span class="text-xs text-muted">SkydashNET</span>
          </div>
        </div>
      </div>

      {/* Right Side - Login Form */}
      <div class="w-full lg:w-1/2 xl:w-2/5 flex items-center justify-center p-6 lg:p-12">
        <div class="w-full max-w-md">
          {/* Mobile Logo */}
          <div class="lg:hidden text-center mb-10">
            <div class="inline-flex items-center gap-3 mb-4">
              <div class="w-12 h-12 relative">
                <svg viewBox="0 0 100 100" class="w-full h-full">
                  <defs>
                    <linearGradient id="logoGradMobile" x1="0%" y1="0%" x2="100%" y2="100%">
                      <stop offset="0%" style="stop-color:#14b8a6" />
                      <stop offset="100%" style="stop-color:#06b6d4" />
                    </linearGradient>
                  </defs>
                  <polygon 
                    points="50,5 95,27.5 95,72.5 50,95 5,72.5 5,27.5" 
                    fill="url(#logoGradMobile)"
                  />
                  <text x="50" y="58" text-anchor="middle" fill="white" font-size="32" font-weight="700" font-family="system-ui">M</text>
                </svg>
              </div>
              <h1 class="text-3xl font-black text-primary">
                mini<span class="text-transparent bg-clip-text bg-gradient-to-r from-teal-500 to-cyan-500">ACS</span>
              </h1>
            </div>
            <p class="text-muted">TR-069 Management System</p>
          </div>

          {/* Form Card */}
          <div class="bg-surface border-2 border-default rounded-2xl p-8 shadow-xl shadow-black/5 transition-theme">
            <div class="mb-8">
              <h2 class="text-2xl font-bold text-primary mb-2">Welcome back</h2>
              <p class="text-secondary text-sm">Masukkan kredensial untuk melanjutkan</p>
            </div>

            <form onSubmit={handleSubmit} class="space-y-6">
              <Show when={error()}>
                <div class="flex items-start gap-3 p-4 rounded-xl bg-rose-500/10 border border-rose-500/20">
                  <AlertCircle size={18} class="text-rose-500 shrink-0 mt-0.5" />
                  <span class="text-rose-500 text-sm">{error()}</span>
                </div>
              </Show>

              <div class="space-y-2">
                <label class="block text-sm font-semibold text-primary">Username</label>
                <input
                  type="text"
                  value={username()}
                  onInput={(e) => setUsername(e.currentTarget.value)}
                  class="w-full px-4 py-3.5 bg-elevated border-2 border-default rounded-xl placeholder-muted focus:outline-none focus:border-teal-500 focus:bg-surface transition-all duration-200"
                  style={{ color: 'rgb(var(--text-primary))' }}
                  placeholder="Masukkan username"
                  required
                  autocomplete="username"
                />
              </div>

              <div class="space-y-2">
                <div class="flex items-center justify-between">
                  <label class="block text-sm font-semibold text-primary">Password</label>
                  <button type="button" class="text-xs text-teal-500 hover:text-teal-400 transition-colors">
                    Lupa password?
                  </button>
                </div>
                <input
                  type="password"
                  value={password()}
                  onInput={(e) => setPassword(e.currentTarget.value)}
                  class="w-full px-4 py-3.5 bg-elevated border-2 border-default rounded-xl placeholder-muted focus:outline-none focus:border-teal-500 focus:bg-surface transition-all duration-200"
                  style={{ color: 'rgb(var(--text-primary))' }}
                  placeholder="Masukkan password"
                  required
                  autocomplete="current-password"
                />
              </div>

              <button
                type="submit"
                disabled={loading()}
                class="w-full py-4 px-6 bg-gradient-to-r from-teal-500 to-cyan-500 text-white font-semibold rounded-xl hover:from-teal-600 hover:to-cyan-600 focus:outline-none focus:ring-2 focus:ring-teal-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200 flex items-center justify-center gap-3 group active:scale-[0.98]"
              >
                <Show 
                  when={loading()} 
                  fallback={
                    <>
                      Login
                      <ArrowRight size={18} class="transition-transform duration-200 group-hover:translate-x-1" />
                    </>
                  }
                >
                  <Loader2 size={18} class="animate-spin" />
                  Logging in...
                </Show>
              </button>
            </form>
          </div>

          {/* Mobile Version Badge */}
          <div class="lg:hidden mt-8 text-center">
            <div class="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-elevated/50">
              <span class="w-1.5 h-1.5 rounded-full bg-emerald-500" />
              <span class="text-xs text-muted">v1.0.0-beta - SkydashNET</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Login;
