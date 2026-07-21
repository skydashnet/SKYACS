import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For } from 'solid-js';
import { Save, Settings as SettingsIcon, Info, Users, Plus, Trash2, Edit2, Key, LogOut } from 'lucide-solid';
import { api, type ProvisioningRule, type User } from '../lib/api';
import PageHeader from '../components/PageHeader';

import { useAuth } from '../lib/auth';

interface SettingField {
  key: string;
  label: string;
  type: 'text' | 'password' | 'number';
  placeholder?: string;
}

const settingFields: SettingField[] = [
  { key: 'firmware_base_url', label: 'Firmware base URL', type: 'text', placeholder: 'https://acs.example.com' },
];

const Settings: Component = () => {
  const { user, isFullAccess, logout } = useAuth();
  const [settings, { refetch }] = createResource(() => api.getSettings());
  const [formData, setFormData] = createSignal<Record<string, string>>({});
  const [saving, setSaving] = createSignal(false);
  const [message, setMessage] = createSignal<{ type: 'success' | 'error'; text: string } | null>(null);

  // User management
  const [users, { refetch: refetchUsers }] = createResource(async () => {
    if (!isFullAccess()) return [];
    return api.getUsers();
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
    return api.getProvisioningRules();
  });

  const [showProvModal, setShowProvModal] = createSignal(false);
  const [editingProv, setEditingProv] = createSignal<ProvisioningRule | null>(null);
  const emptyProvisioningRule = { parameter_name: '', parameter_value: '', parameter_type: 'string', manufacturer: '', product_class: '', enabled: true, description: '' };
  const [provForm, setProvForm] = createSignal({ ...emptyProvisioningRule });
  const handleChange = (key: string, value: string) => {
    setFormData((prev) => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    setSaving(true);
    setMessage(null);

    try {
      await api.updateSettings(formData());
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
    try {
      await api.createUser(userForm());
      setShowUserModal(false);
      setUserForm({ username: '', password: '', role: 'read' });
      refetchUsers();
    } catch (error) { alert((error as Error).message); }
  };

  const handleUpdateUser = async () => {
    const u = editingUser();
    if (!u) return;
    const body: Partial<{ username: string; password: string; role: 'full' | 'read' }> = { username: userForm().username, role: userForm().role };
    if (userForm().password) body.password = userForm().password;
    try {
      await api.updateUser(u.id, body);
      setShowUserModal(false);
      setEditingUser(null);
      setUserForm({ username: '', password: '', role: 'read' });
      refetchUsers();
    } catch (error) { alert((error as Error).message); }
  };

  const handleDeleteUser = async (id: number) => {
    if (!confirm('Yakin hapus user ini?')) return;
    try { await api.deleteUser(id); refetchUsers(); }
    catch (error) { alert((error as Error).message); }
  };

  const handleChangePassword = async () => {
    if (passwordForm().newPass !== passwordForm().confirm) {
      alert('Password baru tidak cocok');
      return;
    }
    try {
      await api.changePassword(passwordForm().current, passwordForm().newPass);
      setShowPasswordModal(false);
      setPasswordForm({ current: '', newPass: '', confirm: '' });
      alert('Password berhasil diubah. Silakan login kembali.');
      logout();
    } catch (error) { alert((error as Error).message); }
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
    try {
      await api.createProvisioningRule(provForm());
      setShowProvModal(false);
      setProvForm({ ...emptyProvisioningRule });
      refetchProvRules();
    } catch (error) { alert((error as Error).message); }
  };

  const handleUpdateProv = async () => {
    const p = editingProv();
    if (!p) return;
    try {
      await api.updateProvisioningRule(p.id, provForm());
      setShowProvModal(false);
      setEditingProv(null);
      refetchProvRules();
    } catch (error) { alert((error as Error).message); }
  };

  const handleDeleteProv = async (id: number) => {
    if (!confirm('Yakin hapus rule ini?')) return;
    try { await api.deleteProvisioningRule(id); refetchProvRules(); }
    catch (error) { alert((error as Error).message); }
  };

  const handleToggleProv = async (id: number, enabled: boolean) => {
    try { await api.toggleProvisioningRule(id, enabled); refetchProvRules(); }
    catch (error) { alert((error as Error).message); }
  };

  const openEditProv = (p: ProvisioningRule) => {
    setEditingProv(p);
    setProvForm({ parameter_name: p.parameter_name, parameter_value: p.parameter_value, parameter_type: p.parameter_type, manufacturer: p.manufacturer || '', product_class: p.product_class || '', enabled: p.enabled, description: p.description });
    setShowProvModal(true);
  };

  const openCreateProv = () => {
    setEditingProv(null);
    setProvForm({ ...emptyProvisioningRule });
    setShowProvModal(true);
  };



  return (
    <div class="space-y-5">
      <PageHeader title="System settings" description="Operator accounts, provisioning, and CWMP defaults.">
        <div class="flex items-center gap-2">
          <span class="text-sm text-muted">
            Logged in as <span class="text-sky-400">{user()?.username}</span> ({user()?.role})
          </span>
          <button onClick={() => { logout(); window.location.href = '/login'; }} class="btn btn-secondary text-xs py-1.5">
            <LogOut size={12} />
            Logout
          </button>
        </div>
      </PageHeader>

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



      {/* Delivery and connection request settings */}
      <div class="card p-5">
        <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
          <SettingsIcon size={14} />
          Delivery & Connection Request
        </h2>
        <p class="text-muted text-xs mb-4">
          Firmware delivery URL dan kredensial yang dipakai SKYACS untuk memanggil CPE.
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
                    disabled={!isFullAccess()}
                  />
                </div>
              )}
            </For>

            {/* Connection Request Credentials Section */}
            <div class="pt-4 border-t border-subtle">
              <div class="flex items-center justify-between mb-3">
                <div>
                  <label class="block text-xs text-muted">Connection Request Credentials</label>
                  <p class="text-xs text-muted mt-0.5">Mode untuk autentikasi connection request ke CPE</p>
                </div>
                <button
                  onClick={() => handleChange('use_auto_conn_credentials', getValue('use_auto_conn_credentials') === 'true' ? 'false' : 'true')}
                  class={`px-3 py-1.5 text-xs font-medium transition-colors ${
                    getValue('use_auto_conn_credentials') === 'true'
                      ? 'bg-sky-500/20 text-sky-400 border border-sky-500/30'
                      : 'bg-zinc-700 text-secondary border border-zinc-600'
                  }`}
                >
                  {getValue('use_auto_conn_credentials') === 'true' ? 'Auto (Serial Number)' : 'Custom'}
                </button>
              </div>

              <Show when={getValue('use_auto_conn_credentials') !== 'true'}>
                <div class="space-y-3 pt-3 border-t border-subtle">
                  <div>
                    <label class="block text-xs text-muted mb-1.5">Connection Request Username</label>
                    <input
                      type="text"
                      value={getValue('connection_request_username')}
                      onInput={(e) => handleChange('connection_request_username', e.currentTarget.value)}
                      placeholder="admin"
                      class="input"
                      disabled={!isFullAccess()}
                    />
                  </div>
                  <div>
                    <label class="block text-xs text-muted mb-1.5">Connection Request Password</label>
                    <input
                      type="password"
                      value={getValue('connection_request_password')}
                      onInput={(e) => handleChange('connection_request_password', e.currentTarget.value)}
                      placeholder="Optional"
                      class="input"
                      disabled={!isFullAccess()}
                    />
                  </div>
                </div>
              </Show>

              <Show when={getValue('use_auto_conn_credentials') === 'true'}>
                <div class="space-y-3 pt-3 border-t border-subtle">
                  <div>
                    <label class="block text-xs text-muted mb-1.5">Connection Request Master Secret</label>
                    <input
                      type="password"
                      value={getValue('connection_request_password')}
                      onInput={(e) => handleChange('connection_request_password', e.currentTarget.value)}
                      placeholder="Minimum 16 characters"
                      class="input"
                      disabled={!isFullAccess()}
                    />
                  </div>
                  <div class="p-3 bg-sky-500/10 border border-sky-500/20 text-xs text-sky-400">
                    <p><strong>Username:</strong> serial number device</p>
                    <p><strong>Password:</strong> HMAC-SHA256 unik per device</p>
                  </div>
                </div>
              </Show>
            </div>
          </div>

          <div class="mt-6 pt-4 border-t border-subtle">
            <Show when={isFullAccess()}><button
              onClick={handleSave}
              disabled={saving()}
              class="btn btn-primary"
            >
              <Save size={14} />
              {saving() ? 'Menyimpan...' : 'Simpan Settings'}
            </button></Show>
          </div>
        </Show>
      </div>

      <div class="card p-5">
        <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
          <Info size={14} />
          CPE Configuration Guide
        </h2>
        <div class="text-muted text-xs space-y-2">
          <p>Untuk connect CPE device ke SKYACS:</p>
          <ol class="list-decimal list-inside space-y-1 ml-2">
            <li>Login ke CPE web interface</li>
            <li>Cari menu TR-069 atau CWMP settings</li>
			<li>Set ACS URL ke endpoint CWMP deployment, misalnya <code class="bg-elevated px-2 py-0.5 rounded text-sky-400">https://cwmp.example.com/</code></li>
			<li>Set username/password dari <code>CWMP_USERNAME</code> dan <code>CWMP_PASSWORD</code> jika autentikasi diaktifkan</li>
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
                  minlength={12}
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
                  minlength={12}
                />
              </div>
              <div>
                <label class="block text-xs text-muted mb-1.5">Password Baru</label>
                <input
                  type="password"
                  value={passwordForm().newPass}
                  onInput={(e) => setPasswordForm(f => ({ ...f, newPass: e.currentTarget.value }))}
                  class="input w-full"
                  minlength={12}
                />
              </div>
              <div>
                <label class="block text-xs text-muted mb-1.5">Konfirmasi Password</label>
                <input
                  type="password"
                  value={passwordForm().confirm}
                  onInput={(e) => setPasswordForm(f => ({ ...f, confirm: e.currentTarget.value }))}
                  class="input w-full"
                  minlength={12}
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
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs text-muted mb-1.5">Manufacturer scope</label>
                  <input
                    type="text"
                    value={provForm().manufacturer}
                    onInput={(e) => setProvForm(f => ({ ...f, manufacturer: e.currentTarget.value }))}
                    class="input w-full"
                    placeholder="Huawei"
                  />
                </div>
                <div>
                  <label class="block text-xs text-muted mb-1.5">Product class scope</label>
                  <input
                    type="text"
                    value={provForm().product_class}
                    onInput={(e) => setProvForm(f => ({ ...f, product_class: e.currentTarget.value }))}
                    class="input w-full"
                    placeholder="HG8145V5"
                  />
                </div>
              </div>
              <div>
                <label class="block text-xs text-muted mb-1.5">CWMP value type</label>
                <select
                  value={provForm().parameter_type}
                  onChange={(e) => setProvForm(f => ({ ...f, parameter_type: e.currentTarget.value }))}
                  class="input w-full"
                >
                  <option value="string">string</option>
                  <option value="boolean">boolean</option>
                  <option value="int">int</option>
                  <option value="unsignedInt">unsignedInt</option>
                  <option value="dateTime">dateTime</option>
                </select>
                <p class="text-xs text-muted mt-1">Rule diterapkan sekali saat BOOTSTRAP dan dicatat per perangkat.</p>
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
