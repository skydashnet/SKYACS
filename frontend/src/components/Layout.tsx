import type { ParentComponent } from 'solid-js';
import { createSignal, For, onCleanup, onMount, Show } from 'solid-js';
import { A, useLocation } from '@solidjs/router';
import {
  Activity, AlertTriangle, ChevronRight, HardDrive, LayoutDashboard, LogOut,
  Menu, Moon, Router, Settings, ShieldCheck, Sun, X,
} from 'lucide-solid';
import { api } from '../lib/api';
import { useAuth } from '../lib/auth';
import { APP_VERSION } from '../lib/version';
import { useTheme } from '../lib/theme';

const navigation = [
  { section: 'Monitor', items: [
    { href: '/', label: 'Fleet status', icon: LayoutDashboard },
    { href: '/devices', label: 'CPE inventory', icon: Router },
    { href: '/faults', label: 'CWMP faults', icon: AlertTriangle },
  ] },
  { section: 'Control', items: [
    { href: '/firmwares', label: 'Firmware library', icon: HardDrive },
    { href: '/security', label: 'Access controls', icon: ShieldCheck, fullOnly: true },
    { href: '/settings', label: 'System settings', icon: Settings },
  ] },
];

const pageNames: Record<string, string> = {
  '/': 'Fleet status',
  '/devices': 'CPE inventory',
  '/faults': 'Fault center',
  '/firmwares': 'Firmware library',
  '/security': 'Security center',
  '/settings': 'System settings',
};

const Brand = () => (
  <div class="brand-lockup">
    <div class="brand-mark" aria-hidden="true">
      <span /><span /><span />
    </div>
    <div>
      <div class="brand-name">SKYACS</div>
      <div class="brand-subtitle">Independent CWMP control plane</div>
    </div>
  </div>
);

const Layout: ParentComponent = (props) => {
  const location = useLocation();
  const [sidebarOpen, setSidebarOpen] = createSignal(false);
  const [apiOnline, setAPIOnline] = createSignal<boolean | null>(null);
  const { isDark, toggleTheme } = useTheme();
  const { user, logout, isFullAccess } = useAuth();
  let menuButton: HTMLButtonElement | undefined;

  const checkHealth = async () => {
    try { await api.health(); setAPIOnline(true); } catch { setAPIOnline(false); }
  };
  onMount(() => {
    checkHealth();
    const timer = window.setInterval(checkHealth, 30_000);
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && sidebarOpen()) {
        setSidebarOpen(false);
        menuButton?.focus();
      }
    };
    document.addEventListener('keydown', closeOnEscape);
    onCleanup(() => { window.clearInterval(timer); document.removeEventListener('keydown', closeOnEscape); });
  });

  const isActive = (href: string) => href === '/' ? location.pathname === '/' : location.pathname.startsWith(href);
  const pageTitle = () => location.pathname.startsWith('/device/') ? 'CPE record' : pageNames[location.pathname] || 'Control plane';

  return (
    <div class="app-shell">
      <Show when={sidebarOpen()}>
        <button class="sidebar-scrim lg:hidden" aria-label="Close navigation" onClick={() => setSidebarOpen(false)} />
      </Show>

      <aside class={`app-sidebar ${sidebarOpen() ? 'is-open' : ''}`}>
        <div class="sidebar-brand">
          <Brand />
          <button class="icon-button lg:hidden" aria-label="Close navigation" onClick={() => setSidebarOpen(false)}><X size={18} /></button>
        </div>

        <nav class="sidebar-nav" aria-label="Primary navigation">
          <For each={navigation}>
            {(group) => (
              <div class="nav-group">
                <p class="nav-section">{group.section}</p>
                <For each={group.items.filter((item) => !item.fullOnly || isFullAccess())}>
                  {(item) => (
                    <A href={item.href} onClick={() => setSidebarOpen(false)} class={`nav-link ${isActive(item.href) ? 'is-active' : ''}`} aria-current={isActive(item.href) ? 'page' : undefined}>
                      <item.icon size={17} stroke-width={1.7} />
                      <span>{item.label}</span>
                      <Show when={isActive(item.href)}><ChevronRight size={13} class="nav-chevron" /></Show>
                    </A>
                  )}
                </For>
              </div>
            )}
          </For>
        </nav>

        <div class="sidebar-footer">
          <div class="operator-card">
            <div class="operator-avatar">{user()?.username?.slice(0, 2).toUpperCase()}</div>
            <div class="operator-copy">
              <strong>{user()?.username}</strong>
              <span>{isFullAccess() ? 'Full access' : 'Read only'}</span>
            </div>
            <button class="icon-button" onClick={logout} aria-label="Sign out" title="Sign out"><LogOut size={16} /></button>
          </div>
          <div class="sidebar-meta"><span>SkydashNET</span><span>v{APP_VERSION}</span></div>
        </div>
      </aside>

      <div class="app-main">
        <header class="topbar">
          <div class="topbar-title">
            <button ref={menuButton} class="icon-button lg:hidden" aria-label="Open navigation" aria-expanded={sidebarOpen()} onClick={() => setSidebarOpen(true)}><Menu size={20} /></button>
            <div>
              <p>SKYACS <span>/</span></p>
              <h1>{pageTitle()}</h1>
            </div>
          </div>
          <div class="topbar-actions">
            <button type="button" class={`service-state ${apiOnline() === false ? 'is-down' : ''}`} onClick={checkHealth} aria-live="polite" aria-label="Refresh API service status">
              <Activity size={14} />
              <span>{apiOnline() === null ? 'Checking API' : apiOnline() ? 'API operational' : 'API unavailable'}</span>
            </button>
            <button class="icon-button" onClick={toggleTheme} aria-label={isDark() ? 'Use light theme' : 'Use dark theme'}>
              {isDark() ? <Sun size={17} /> : <Moon size={17} />}
            </button>
          </div>
        </header>

        <main class="content-area">{props.children}</main>
      </div>
    </div>
  );
};

export default Layout;
