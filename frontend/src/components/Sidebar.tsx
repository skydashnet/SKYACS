import type { Component } from 'solid-js';
import { A, useLocation } from '@solidjs/router';
import { LayoutDashboard, Router, AlertTriangle, HardDrive, Settings, Sun, Moon } from 'lucide-solid';
import { useTheme } from '../lib/theme';

const navItems = [
  { href: '/', label: 'Dashboard', icon: LayoutDashboard },
  { href: '/devices', label: 'Devices', icon: Router },
  { href: '/faults', label: 'Faults', icon: AlertTriangle },
  { href: '/firmwares', label: 'Firmwares', icon: HardDrive },
  { href: '/settings', label: 'Settings', icon: Settings }
];

const Sidebar: Component = () => {
  const location = useLocation();
  const { isDark, toggleTheme } = useTheme();

  const isActive = (href: string) => {
    if (href === '/') return location.pathname === '/';
    return location.pathname.startsWith(href);
  };

  return (
    <aside class="sidebar w-56 flex flex-col h-screen">
      <div class="p-5 border-b border-subtle">
        <div class="flex items-center justify-between">
          <div>
            <h1 class="text-lg font-semibold text-primary">miniACS</h1>
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
      </div>

      <nav class="flex-1 p-3 overflow-y-auto">
        <ul class="space-y-1">
          {navItems.map((item) => (
            <li>
              <A
                href={item.href}
                class={`sidebar-link flex items-center gap-3 px-3 py-2 rounded-md text-sm ${
                  isActive(item.href) ? 'active' : ''
                }`}
              >
                <item.icon size={18} stroke-width={1.5} />
                {item.label}
              </A>
            </li>
          ))}
        </ul>
      </nav>

      <div class="p-4 border-t border-subtle">
        <p class="text-xs text-muted">SkydashNET</p>
        <p class="text-[10px] text-muted opacity-50 mt-0.5">v1.0.0-beta</p>
      </div>
    </aside>
  );
};

export default Sidebar;
