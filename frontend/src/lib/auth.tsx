import { createSignal, createContext, useContext, type ParentComponent, type Accessor } from 'solid-js';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:7547/api';

export interface User {
  id: number;
  username: string;
  role: 'full' | 'read';
  created_at: string;
  last_login: string | null;
}

interface AuthContextType {
  user: Accessor<User | null>;
  token: Accessor<string | null>;
  isAuthenticated: Accessor<boolean>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  isFullAccess: Accessor<boolean>;
}

const AuthContext = createContext<AuthContextType>();

export const AuthProvider: ParentComponent = (props) => {
  const [user, setUser] = createSignal<User | null>(null);
  const [token, setToken] = createSignal<string | null>(localStorage.getItem('token'));

  const isAuthenticated = () => !!token();
  const isFullAccess = () => user()?.role === 'full';

  // Try to restore user from token
  if (token()) {
    fetch(`${API_BASE}/auth/me`, {
      headers: { Authorization: `Bearer ${token()}` }
    })
      .then(res => {
        if (!res.ok) throw new Error('Invalid token');
        return res.json();
      })
      .then(data => setUser(data))
      .catch(() => {
        localStorage.removeItem('token');
        setToken(null);
      });
  }

  const login = async (username: string, password: string) => {
    const res = await fetch(`${API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });

    if (!res.ok) {
      const data = await res.json();
      throw new Error(data.error || 'Login gagal');
    }

    const data = await res.json();
    setToken(data.token);
    setUser(data.user);
    localStorage.setItem('token', data.token);
  };

  const logout = () => {
    setToken(null);
    setUser(null);
    localStorage.removeItem('token');
    window.location.href = '/login';
  };

  return (
    <AuthContext.Provider value={{ user, token, isAuthenticated, login, logout, isFullAccess }}>
      {props.children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
};

export const getAuthHeaders = (): Record<string, string> => {
  const token = localStorage.getItem('token');
  return token ? { Authorization: `Bearer ${token}` } : {};
};
