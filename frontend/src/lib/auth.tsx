import { createContext, createSignal, onMount, useContext, type Accessor, type ParentComponent } from 'solid-js';
import { API_BASE, clearStoredSession, getStoredToken, request, type User } from './api';

interface AuthContextType {
  user: Accessor<User | null>;
  token: Accessor<string | null>;
  ready: Accessor<boolean>;
  isAuthenticated: Accessor<boolean>;
  isFullAccess: Accessor<boolean>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType>();

export const AuthProvider: ParentComponent = (props) => {
  const [user, setUser] = createSignal<User | null>(null);
  const [token, setToken] = createSignal<string | null>(getStoredToken());
  const [ready, setReady] = createSignal(false);

  const isAuthenticated = () => Boolean(token());
  const isFullAccess = () => user()?.role === 'full';

  onMount(async () => {
    // Remove legacy persistent tokens: authentication should not survive a closed browser session.
    localStorage.removeItem('token');
    if (token()) {
      try {
        setUser(await request<User>('/auth/me'));
      } catch {
        clearStoredSession();
        setToken(null);
      }
    }
    setReady(true);
  });

  const login = async (username: string, password: string) => {
    const response = await fetch(`${API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    const body = await response.json().catch(() => ({ error: 'Login service unavailable' }));
    if (!response.ok) throw new Error(body.error || 'Login failed');

    sessionStorage.setItem('skyacs_token', body.token);
    setToken(body.token);
    setUser(body.user);
  };

  const logout = () => {
    clearStoredSession();
    setToken(null);
    setUser(null);
    window.location.assign('/login');
  };

  return (
    <AuthContext.Provider value={{ user, token, ready, isAuthenticated, isFullAccess, login, logout }}>
      {props.children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used within AuthProvider');
  return context;
};
