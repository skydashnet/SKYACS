import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For } from 'solid-js';
import { Save, Settings as SettingsIcon, Info, Users, Plus, Trash2, Edit2, Key, LogOut } from 'lucide-solid';
import { api } from '../lib/api';
import { useAuth } from '../lib/auth';

interface SettingField {
  key: string;
  label: string;
  type: 'text' | 'password' | 'number';
  placeholder?: string;
}

interface User {
  id: number;
  username: string;
  role: 'full' | 'read';
  created_at: string;
  last_login: string | null;
}

interface ProvisioningRule {
  id: number;
  parameter_name: string;
  parameter_value: string;
  parameter_type: string;
  enabled: boolean;
  description: string;
}

const settingFields: SettingField[] = [
  { key: 'server_url', label: 'Server URL', type: 'text', placeholder: 'http://192.168.1.100:7547' },
  { key: 'acs_username', label: 'ACS Username', type: 'text', placeholder: 'Optional' },
  { key: 'acs_password', label: 'ACS Password', type: 'password', placeholder: 'Optional' },
  { key: 'inform_interval', label: 'Inform Interval (seconds)', type: 'number', placeholder: '3600' },
  { key: 'connection_request_username', label: 'Connection Request Username', type: 'text', placeholder: 'admin' },
  { key: 'connection_request_password', label: 'Connection Request Password', type: 'password', placeholder: 'Optional' },
];

const API_BASE = import.meta.env.VITE_API_URL;

const Settings: Component = () => {
  const { user, isFullAccess, logout } = useAuth();
  const [settings, { refetch }] = createResource(() => api.getSettings());
  const [formData, setFormData] = createSignal<Record<string, string>>({});
  const [saving, setSaving] = createSignal(false);
  const [message, setMessage] = createSignal<{ type: 'success' | 'error'; text: string } | null>(null);

  // User management
  const [users, { refetch: refetchUsers }] = createResource(async () => {
    if (!isFullAccess()) return [];
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_BASE}/users`, {
      headers: { Authorization: `Bearer ${token}` }
    });
    if (!res.ok) return [];
    return res.json() as Promise<User[]>;
  });

  const [showUserModal, setShowUserModal] = createSignal(false);
  const [editingUser, setEditingUser] = createSignal<User | null>(null);
  const [userForm, setUserForm] = createSignal({ username: '', password: '', role: 'read' as 'full' | 'read' });

  // Change password
  const [showPasswordModal, setShowPasswordModal] = createSignal(false);
  const [passwordForm, setPasswordForm] = createSignal({ current: '', newPass: '', confirm: '' });

  // Provisioning rules
  const [provRules, { refetch: refetchProvRules }] = createResource(async () => {
    if (!isFullAccess()) return [];
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_BASE}/provisioning`, {
      headers: { Authorization: `Bearer ${token}` }
    });
    if (!res.ok) return [];
    return res.json() as Promise<ProvisioningRule[]>;
  });

  const [showProvModal, setShowProvModal] = createSignal(false);
  const [editingProv, setEditingProv] = createSignal<ProvisioningRule | null>(null);
  const [provForm, setProvForm] = createSignal({ parameter_name: '', parameter_value: '', parameter_type: 'string', enabled: true, description: '' });

  const handleChange = (key: string, value: string) => {
    setFormData((prev) => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    setSaving(true);
    setMessage(null);

    try {
      const data = { ...settings(), ...formData() };
      await api.updateSettings(data);
      setMessage({ type: 'success', text: 'Settings berhasil disimpan!' });
      refetch();
      setFormData({});
    } catch (err) {
      setMessage({ type: 'error', text: 'Gagal menyimpan: ' + (err as Error).message });
    } finally {
      setSaving(false);
    }
  };

  const getValue = (key: string) => {
    return formData()[key] ?? settings()?.[key] ?? '';
  };

  const handleCreateUser = async () => {
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_BASE}/users`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify(userForm())
    });
    if (res.ok) {
      setShowUserModal(false);
      setUserForm({ username: '', password: '', role: 'read' });
      refetchUsers();
    } else {
      const data = await res.json();
      alert(data.error || 'Gagal membuat user');
    }
  };

  const handleUpdateUser = async () => {
    const u = editingUser();
    if (!u) return;
    const token = localStorage.getItem('token');
    const body: Record<string, string> = { username: userForm().username, role: userForm().role };
    if (userForm().password) body.password = userForm().password;
    
    const res = await fetch(`${API_BASE}/users/${u.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify(body)
    });
    if (res.ok) {
      setShowUserModal(false);
      setEditingUser(null);
      setUserForm({ username: '', password: '', role: 'read' });
      refetchUsers();
    } else {
      const data = await res.json();
      alert(data.error || 'Gagal update user');
    }
  };

  const handleDeleteUser = async (id: number) => {
    if (!confirm('Yakin hapus user ini?')) return;
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_BASE}/users/${id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` }
    });
    if (res.ok) {
      refetchUsers();
    } else {
      const data = await res.json();
      alert(data.error || 'Gagal hapus user');
    }
  };

  const handleChangePassword = async () => {
    if (passwordForm().newPass !== passwordForm().confirm) {
      alert('Password baru tidak cocok');
      return;
    }
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_BASE}/auth/change-password`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ current_password: passwordForm().current, new_password: passwordForm().newPass })
    });
    if (res.ok) {
      setShowPasswordModal(false);
      setPasswordForm({ current: '', newPass: '', confirm: '' });
      alert('Password berhasil diubah');
    } else {
      const data = await res.json();
      alert(data.error || 'Gagal ubah password');
    }
  };

  const openEditUser = (u: User) => {
    setEditingUser(u);
    setUserForm({ username: u.username, password: '', role: u.role });
    setShowUserModal(true);
  };

  const openCreateUser = () => {
    setEditingUser(null);
    setUserForm({ username: '', password: '', role: 'read' });
    setShowUserModal(true);
  };

  // Provisioning handlers
  const handleCreateProv = async () => {
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_BASE}/provisioning`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify(provForm())
    });
    if (res.ok) {
      setShowProvModal(false);
      setProvForm({ parameter_name: '', parameter_value: '', parameter_type: 'string', enabled: true, description: '' });
      refetchProvRules();
    } else {
      const data = await res.json();
      alert(data.error || 'Gagal membuat rule');
    }
  };

  const handleUpdateProv = async () => {
    const p = editingProv();
    if (!p) return;
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_BASE}/provisioning/${p.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ ...provForm(), id: p.id })
    });
    if (res.ok) {
      setShowProvModal(false);
      setEditingProv(null);
      refetchProvRules();
    } else {
      const data = await res.json();
      alert(data.error || 'Gagal update rule');
    }
  };

  const handleDeleteProv = async (id: number) => {
    if (!confirm('Yakin hapus rule ini?')) return;
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_BASE}/provisioning/${id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` }
    });
    if (res.ok) {
      refetchProvRules();
    }
  };

  const handleToggleProv = async (id: number, enabled: boolean) => {
    const token = localStorage.getItem('token');
    await fetch(`${API_BASE}/provisioning/${id}/toggle`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ enabled })
    });
    refetchProvRules();
  };

  const openEditProv = (p: ProvisioningRule) => {
    setEditingProv(p);
    setProvForm({ parameter_name: p.parameter_name, parameter_value: p.parameter_value, parameter_type: p.parameter_type, enabled: p.enabled, description: p.description });
    setShowProvModal(true);
  };

  const openCreateProv = () => {
    setEditingProv(null);
    setProvForm({ parameter_name: '', parameter_value: '', parameter_type: 'string', enabled: true, description: '' });
    setShowProvModal(true);
  };

  return (
    <div class="space-y-5">
      <div class="flex items-center justify-between">
        <h1 class="text-xl font-semibold text-primary">Settings</h1>
        <div class="flex items-center gap-2">
          <span class="text-sm text-muted">
            Logged in as <span class="text-teal-400">{user()?.username}</span> ({user()?.role})
          </span>
          <button onClick={() => { logout(); window.location.href = '/login'; }} class="btn btn-secondary text-xs py-1.5">
            <LogOut size={12} />
            Logout
          </button>
        </div>
      </div>

      <Show when={message()}>
        <div class={`p-3 rounded-md text-sm ${message()?.type === 'success' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'}`}>
          {message()?.text}
        </div>
      </Show>

      {/* Account Section */}
      <div class="card p-5">
        <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
          <Key size={14} />
          Account
        </h2>
        <button onClick={() => setShowPasswordModal(true)} class="btn btn-secondary">
          <Key size={14} />
          Ganti Password
        </button>
      </div>

      {/* User Management - Only for full access */}
      <Show when={isFullAccess()}>
        <div class="card p-5">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-sm font-medium text-secondary flex items-center gap-2">
              <Users size={14} />
              User Management
            </h2>
            <button onClick={openCreateUser} class="btn btn-primary text-xs py-1.5">
              <Plus size={12} />
              Tambah User
            </button>
          </div>

          <Show when={!users.loading && (users()?.length ?? 0) > 0}>
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-subtle">
                  <th class="text-left py-2 text-xs text-muted font-medium">Username</th>
                  <th class="text-left py-2 text-xs text-muted font-medium">Role</th>
                  <th class="text-left py-2 text-xs text-muted font-medium">Last Login</th>
                  <th class="text-right py-2 text-xs text-muted font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                <For each={users()}>
                  {(u) => (
                    <tr class="border-b border-subtle/50">
                      <td class="py-2 text-primary">{u.username}</td>
                      <td class="py-2">
                        <span class={`badge ${u.role === 'full' ? 'badge-success' : 'badge-warning'}`}>
                          {u.role}
                        </span>
                      </td>
                      <td class="py-2 text-muted text-xs">
                        {u.last_login ? new Date(u.last_login).toLocaleString('id-ID') : 'Never'}
                      </td>
                      <td class="py-2 text-right">
                        <div class="flex items-center justify-end gap-1">
                          <button onClick={() => openEditUser(u)} class="p-1.5 rounded hover:bg-zinc-700 text-muted hover:text-secondary">
                            <Edit2 size={14} />
                          </button>
                          <button onClick={() => handleDeleteUser(u.id)} class="p-1.5 rounded hover:bg-rose-500/20 text-muted hover:text-rose-400">
                            <Trash2 size={14} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </Show>
        </div>

        {/* Auto Provisioning */}
        <div class="card p-5 mt-5">
          <div class="flex items-center justify-between mb-4">
            <div>
              <h2 class="text-sm font-medium text-secondary flex items-center gap-2">
                <SettingsIcon size={14} />
                Auto Provisioning
              </h2>
              <p class="text-muted text-xs mt-1">Parameter yang akan otomatis di-push ke setiap device saat connect.</p>
            </div>
            <button onClick={openCreateProv} class="btn btn-primary text-xs py-1.5">
              <Plus size={12} />
              Tambah Rule
            </button>
          </div>

          <Show when={!provRules.loading && (provRules()?.length ?? 0) > 0}>
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-subtle">
                  <th class="text-left py-2 text-xs text-muted font-medium">Parameter</th>
                  <th class="text-left py-2 text-xs text-muted font-medium">Value</th>
                  <th class="text-left py-2 text-xs text-muted font-medium">Status</th>
                  <th class="text-right py-2 text-xs text-muted font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                <For each={provRules()}>
                  {(p) => (
                    <tr class="border-b border-subtle/50">
                      <td class="py-2">
                        <span class="text-primary text-xs font-mono">{p.parameter_name}</span>
                        <Show when={p.description}>
                          <p class="text-muted text-xs">{p.description}</p>
                        </Show>
                      </td>
                      <td class="py-2 text-secondary text-xs font-mono max-w-xs truncate">{p.parameter_value}</td>
                      <td class="py-2">
                        <button
                          onClick={() => handleToggleProv(p.id, !p.enabled)}
                          class={`px-2 py-0.5 rounded text-xs ${p.enabled ? 'bg-emerald-500/20 text-emerald-400' : 'bg-zinc-700 text-secondary'}`}
                        >
                          {p.enabled ? 'Active' : 'Disabled'}
                        </button>
                      </td>
                      <td class="py-2 text-right">
                        <div class="flex items-center justify-end gap-1">
                          <button onClick={() => openEditProv(p)} class="p-1.5 rounded hover:bg-zinc-700 text-muted hover:text-secondary">
                            <Edit2 size={14} />
                          </button>
                          <button onClick={() => handleDeleteProv(p.id)} class="p-1.5 rounded hover:bg-rose-500/20 text-muted hover:text-rose-400">
                            <Trash2 size={14} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </Show>

          <Show when={!provRules.loading && (provRules()?.length ?? 0) === 0}>
            <p class="text-muted text-sm text-center py-4">Belum ada provisioning rules.</p>
          </Show>
        </div>
      </Show>

      {/* ACS Settings */}
      <div class="card p-5">
        <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
          <SettingsIcon size={14} />
          ACS Configuration
        </h2>
        <p class="text-muted text-xs mb-4">
          Konfigurasi URL untuk CPE connection. Bisa menggunakan IP atau domain.
        </p>

        <Show when={!settings.loading} fallback={
          <div class="space-y-4">
            <div class="skeleton h-10 w-full" />
            <div class="skeleton h-10 w-full" />
            <div class="skeleton h-10 w-full" />
          </div>
        }>
          <div class="space-y-4">
            <For each={settingFields}>
              {(field) => (
                <div>
                  <label class="block text-xs text-muted mb-1.5">
                    {field.label}
                  </label>
                  <input
                    type={field.type}
                    value={getValue(field.key)}
                    onInput={(e) => handleChange(field.key, e.currentTarget.value)}
                    placeholder={field.placeholder}
                    class="input"
                  />
                </div>
              )}
            </For>
          </div>

          <div class="mt-6 pt-4 border-t border-subtle">
            <button
              onClick={handleSave}
              disabled={saving()}
              class="btn btn-primary"
            >
              <Save size={14} />
              {saving() ? 'Menyimpan...' : 'Simpan Settings'}
            </button>
          </div>
        </Show>
      </div>

      <div class="card p-5">
        <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
          <Info size={14} />
          CPE Configuration Guide
        </h2>
        <div class="text-muted text-xs space-y-2">
          <p>Untuk connect CPE device ke miniACS:</p>
          <ol class="list-decimal list-inside space-y-1 ml-2">
            <li>Login ke CPE web interface</li>
            <li>Cari menu TR-069 atau CWMP settings</li>
            <li>Set ACS URL ke: <code class="bg-elevated px-2 py-0.5 rounded text-teal-400">{getValue('server_url') || 'http://your-server:7547/'}</code></li>
            <li>Set username/password jika diperlukan</li>
            <li>Save dan CPE akan auto-connect</li>
          </ol>
        </div>
      </div>

      {/* User Modal */}
      <Show when={showUserModal()}>
        <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowUserModal(false)}>
          <div class="card p-5 w-full max-w-sm" onClick={(e) => e.stopPropagation()}>
            <h3 class="text-lg font-semibold text-primary mb-4">
              {editingUser() ? 'Edit User' : 'Tambah User'}
            </h3>
            <div class="space-y-4">
              <div>
                <label class="block text-xs text-muted mb-1.5">Username</label>
                <input
                  type="text"
                  value={userForm().username}
                  onInput={(e) => setUserForm(f => ({ ...f, username: e.currentTarget.value }))}
                  class="input w-full"
                  placeholder="Username"
                />
              </div>
              <div>
                <label class="block text-xs text-muted mb-1.5">
                  Password {editingUser() && '(kosongkan jika tidak diubah)'}
                </label>
                <input
                  type="password"
                  value={userForm().password}
                  onInput={(e) => setUserForm(f => ({ ...f, password: e.currentTarget.value }))}
                  class="input w-full"
                  placeholder="Password"
                />
              </div>
              <div>
                <label class="block text-xs text-muted mb-1.5">Role</label>
                <select
                  value={userForm().role}
                  onChange={(e) => setUserForm(f => ({ ...f, role: e.currentTarget.value as 'full' | 'read' }))}
                  class="input w-full"
                >
                  <option value="read">Read Only</option>
                  <option value="full">Full Access</option>
                </select>
              </div>
              <div class="flex gap-2 pt-2">
                <button onClick={() => setShowUserModal(false)} class="btn btn-secondary flex-1">
                  Batal
                </button>
                <button onClick={editingUser() ? handleUpdateUser : handleCreateUser} class="btn btn-primary flex-1">
                  {editingUser() ? 'Update' : 'Tambah'}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Show>

      {/* Password Modal */}
      <Show when={showPasswordModal()}>
        <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowPasswordModal(false)}>
          <div class="card p-5 w-full max-w-sm" onClick={(e) => e.stopPropagation()}>
            <h3 class="text-lg font-semibold text-primary mb-4">Ganti Password</h3>
            <div class="space-y-4">
              <div>
                <label class="block text-xs text-muted mb-1.5">Password Saat Ini</label>
                <input
                  type="password"
                  value={passwordForm().current}
                  onInput={(e) => setPasswordForm(f => ({ ...f, current: e.currentTarget.value }))}
                  class="input w-full"
                />
              </div>
              <div>
                <label class="block text-xs text-muted mb-1.5">Password Baru</label>
                <input
                  type="password"
                  value={passwordForm().newPass}
                  onInput={(e) => setPasswordForm(f => ({ ...f, newPass: e.currentTarget.value }))}
                  class="input w-full"
                />
              </div>
              <div>
                <label class="block text-xs text-muted mb-1.5">Konfirmasi Password</label>
                <input
                  type="password"
                  value={passwordForm().confirm}
                  onInput={(e) => setPasswordForm(f => ({ ...f, confirm: e.currentTarget.value }))}
                  class="input w-full"
                />
              </div>
              <div class="flex gap-2 pt-2">
                <button onClick={() => setShowPasswordModal(false)} class="btn btn-secondary flex-1">
                  Batal
                </button>
                <button onClick={handleChangePassword} class="btn btn-primary flex-1">
                  Ubah Password
                </button>
              </div>
            </div>
          </div>
        </div>
      </Show>

      {/* Provisioning Modal */}
      <Show when={showProvModal()}>
        <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowProvModal(false)}>
          <div class="card p-5 w-full max-w-md" onClick={(e) => e.stopPropagation()}>
            <h3 class="text-lg font-semibold text-primary mb-4">
              {editingProv() ? 'Edit Provisioning Rule' : 'Tambah Provisioning Rule'}
            </h3>
            <div class="space-y-4">
              <div>
                <label class="block text-xs text-muted mb-1.5">Parameter Name</label>
                <input
                  type="text"
                  value={provForm().parameter_name}
                  onInput={(e) => setProvForm(f => ({ ...f, parameter_name: e.currentTarget.value }))}
                  class="input w-full font-mono text-sm"
                  placeholder="InternetGatewayDevice.ManagementServer.URL"
                />
              </div>
              <div>
                <label class="block text-xs text-muted mb-1.5">Parameter Value</label>
                <input
                  type="text"
                  value={provForm().parameter_value}
                  onInput={(e) => setProvForm(f => ({ ...f, parameter_value: e.currentTarget.value }))}
                  class="input w-full"
                  placeholder="http://acs.example.com"
                />
              </div>
              <div>
                <label class="block text-xs text-muted mb-1.5">Description (opsional)</label>
                <input
                  type="text"
                  value={provForm().description}
                  onInput={(e) => setProvForm(f => ({ ...f, description: e.currentTarget.value }))}
                  class="input w-full"
                  placeholder="Migrasi ACS URL"
                />
              </div>
              <div class="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={provForm().enabled}
                  onChange={(e) => setProvForm(f => ({ ...f, enabled: e.currentTarget.checked }))}
                  class="rounded"
                />
                <label class="text-sm text-secondary">Aktifkan rule ini</label>
              </div>
              <div class="flex gap-2 pt-2">
                <button onClick={() => setShowProvModal(false)} class="btn btn-secondary flex-1">
                  Batal
                </button>
                <button onClick={editingProv() ? handleUpdateProv : handleCreateProv} class="btn btn-primary flex-1">
                  {editingProv() ? 'Update' : 'Tambah'}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};

export default Settings;
