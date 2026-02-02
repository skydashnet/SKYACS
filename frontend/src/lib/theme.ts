import { createSignal, createEffect } from 'solid-js';

type Theme = 'dark' | 'light';

const getInitialTheme = (): Theme => {
  if (typeof window !== 'undefined') {
    const stored = localStorage.getItem('miniacs-theme');
    if (stored === 'light' || stored === 'dark') return stored;
  }
  return 'dark';
};

const [theme, setTheme] = createSignal<Theme>(getInitialTheme());

createEffect(() => {
  const currentTheme = theme();
  document.documentElement.setAttribute('data-theme', currentTheme);
  localStorage.setItem('miniacs-theme', currentTheme);
});

export const useTheme = () => {
  return {
    theme,
    setTheme,
    toggleTheme: () => setTheme(t => t === 'dark' ? 'light' : 'dark'),
    isDark: () => theme() === 'dark',
    isLight: () => theme() === 'light',
  };
};
