import { createSignal } from 'solid-js';

type Theme = 'dark' | 'light';

const getInitialTheme = (): Theme => {
  if (typeof window !== 'undefined') {
    const stored = localStorage.getItem('skyacs-theme');
    if (stored === 'light' || stored === 'dark') return stored;
    if (window.matchMedia('(prefers-color-scheme: light)').matches) return 'light';
  }
  return 'dark';
};

const [theme, setThemeSignal] = createSignal<Theme>(getInitialTheme());

const applyTheme = (currentTheme: Theme) => {
  document.documentElement.setAttribute('data-theme', currentTheme);
  localStorage.setItem('skyacs-theme', currentTheme);
};

if (typeof document !== 'undefined') applyTheme(theme());

const setTheme = (nextTheme: Theme) => {
  setThemeSignal(nextTheme);
  applyTheme(nextTheme);
};

export const useTheme = () => {
  return {
    theme,
    setTheme,
    toggleTheme: () => setTheme(theme() === 'dark' ? 'light' : 'dark'),
    isDark: () => theme() === 'dark',
    isLight: () => theme() === 'light',
  };
};
