import type { ParentComponent } from 'solid-js';
import { createSignal, Show } from 'solid-js';
import { A, useLocation } from '@solidjs/router';
import { LayoutDashboard, Router, AlertTriangle, HardDrive, Settings, Menu, X, Sun, Moon } from 'lucide-solid';
import { useTheme } from '../lib/theme';
import { useAuth } from '../lib/auth.tsx';

const navItems = [
  { href: '/', label: 'Dashboard', icon: LayoutDashboard },
  { href: '/devices', label: 'Devices', icon: Router },
  { href: '/faults', label: 'Faults', icon: AlertTriangle },
  { href: '/firmwares', label: 'Firmwares', icon: HardDrive },
  { href: '/settings', label: 'Settings', icon: Settings }
];

const Layout: ParentComponent = (props) => {
  const location = useLocation();
  const [sidebarOpen, setSidebarOpen] = createSignal(false);
  const { isDark, toggleTheme } = useTheme();
  const { logout } = useAuth();

  const isActive = (href: string) => {
    if (href === '/') return location.pathname === '/';
    return location.pathname.startsWith(href);
  };

  const closeSidebar = () => setSidebarOpen(false);

  return (
    <div class="flex h-screen bg-base transition-theme">
      {/* Mobile Header */}
      <div class="lg:hidden fixed top-0 left-0 right-0 h-14 bg-surface border-b border-subtle flex items-center justify-between px-4 z-40 transition-theme">
        <div class="flex items-center">
          <button onClick={() => setSidebarOpen(true)} class="p-2 text-secondary hover:text-primary transition-fast">
            <Menu size={22} />
          </button>
          <h1 class="ml-3 text-lg font-semibold text-primary">miniACS</h1>
          <p class="text-xs text-muted mt-0.5">TR-069 Management</p>
        </div>
        <button 
          onClick={toggleTheme}
          class="theme-toggle"
          title={isDark() ? 'Switch to Light Mode' : 'Switch to Dark Mode'}
        >
          <div class="theme-toggle-knob">
            {isDark() ? <Moon size={12} /> : <Sun size={12} />}
          </div>
        </button>
      </div>

      {/* Mobile Overlay */}
      <Show when={sidebarOpen()}>
        <div class="lg:hidden fixed inset-0 bg-black/60 z-40" onClick={closeSidebar} />
      </Show>

      {/* Sidebar */}
      <aside class={`
        sidebar fixed lg:relative inset-y-0 left-0 z-50
        w-64 lg:w-52 flex flex-col
        transform transition-all duration-200 ease-in-out
        ${sidebarOpen() ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
      `}>
        {/* Header with theme toggle top-right */}
        <div class="p-4 border-b border-subtle flex items-center justify-between">
          <div>
            <h1 class="text-base font-semibold text-primary">miniACS</h1>
            <p class="text-[10px] text-muted mt-0.5">TR-069 Management</p>
          </div>
          <div class="flex items-center gap-1">
            <button 
              onClick={toggleTheme}
              class="p-1.5 rounded-md hover:bg-elevated text-muted hover:text-primary transition-fast hidden lg:block"
              title={isDark() ? 'Light Mode' : 'Dark Mode'}
            >
              {isDark() ? <Sun size={14} /> : <Moon size={14} />}
            </button>
            <button onClick={closeSidebar} class="lg:hidden p-1.5 text-secondary hover:text-primary transition-fast">
              <X size={18} />
            </button>
          </div>
        </div>

        {/* Navigation with grouped items */}
        <nav class="flex-1 px-2 py-3 overflow-y-auto">
          {/* Main Navigation */}
          <ul class="space-y-0.5">
            {navItems.slice(0, 4).map((item) => (
              <li>
                <A
                  href={item.href}
                  onClick={closeSidebar}
                  class={`flex items-center gap-2.5 px-3 py-2 rounded-md text-sm transition-fast relative ${
                    isActive(item.href) 
                      ? 'bg-teal-500/10 text-teal-500 font-medium before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-teal-500 before:rounded-full' 
                      : 'text-secondary hover:bg-elevated hover:text-primary'
                  }`}
                >
                  <item.icon size={16} stroke-width={1.5} />
                  {item.label}
                </A>
              </li>
            ))}
          </ul>

          {/* Divider */}
          <div class="my-3 border-t border-subtle" />

          {/* Settings */}
          <ul class="space-y-0.5">
            {navItems.slice(4).map((item) => (
              <li>
                <A
                  href={item.href}
                  onClick={closeSidebar}
                  class={`flex items-center gap-2.5 px-3 py-2 rounded-md text-sm transition-fast relative ${
                    isActive(item.href) 
                      ? 'bg-teal-500/10 text-teal-500 font-medium before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-teal-500 before:rounded-full' 
                      : 'text-secondary hover:bg-elevated hover:text-primary'
                  }`}
                >
                  <item.icon size={16} stroke-width={1.5} />
                  {item.label}
                </A>
              </li>
            ))}
          </ul>
        </nav>

        {/* Footer */}
        <div class="p-3 border-t border-subtle space-y-2">
          <button
            onClick={logout}
            class="w-full flex items-center gap-2 px-3 py-1.5 rounded-md text-xs text-rose-400 hover:bg-rose-500/10 transition-fast"
          >
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
            Logout
          </button>
          <div class="flex items-center justify-between text-[10px] text-muted/60">
            <span>SkydashNET</span>
            <span>v1.0.0-beta</span>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main class="flex-1 overflow-auto pt-14 lg:pt-0 bg-base transition-theme">
        <div class="p-4 lg:p-6">
          {props.children}
        </div>
      </main>
    </div>
  );
};

export default Layout;
