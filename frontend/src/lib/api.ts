export const API_BASE = import.meta.env.VITE_API_URL || `${window.location.protocol}//${window.location.hostname}:7548`;

export interface Device {
  id: number;
  serial_number: string;
  oui: string;
  manufacturer: string | null;
  product_class: string | null;
  model_name: string | null;
  hardware_version: string | null;
  software_version: string | null;
  ip_address: string | null;
  connection_request_url: string | null;
  last_inform: string | null;
  online: boolean;
  created_at: string;
  updated_at: string;
  parameters?: DeviceParameter[];
}

export interface DeviceStats { total: number; online: number; offline: number }
export interface DeviceListResponse { devices: Device[]; total: number; limit: number; offset: number }
export interface DeviceParameter { id: number; device_id: number; name: string; value: string; writable?: boolean; updated_at: string }
export interface Task { id: number; device_id: number; type: string; payload: unknown; status: string; result: unknown; error_message?: string; created_at: string; sent_at?: string; completed_at?: string }
export interface Firmware { id: number; filename: string; version: string; manufacturer?: string; product_class?: string; file_size: number; checksum?: string; description?: string; created_at: string; updated_at: string }
export interface Fault { id: number; device_id: number; serial_number: string; fault_code: string; fault_string: string; parameter_name: string; resolved: boolean; created_at: string; resolved_at: string | null }
export interface User { id: number; username: string; role: 'full' | 'read'; created_at: string; last_login: string | null }
export interface ProvisioningRule { id: number; parameter_name: string; parameter_value: string; parameter_type: string; enabled: boolean; description: string }
export interface AuditLog { id: number; user_id?: number; username: string; action: string; resource: string; status: number; ip_address: string; user_agent?: string; created_at: string }
export interface BlockedDevice { id: number; serial_number: string; reason: string; created_by: string; created_at: string }

export const getStoredToken = () => sessionStorage.getItem('miniacs_token');

export const clearStoredSession = () => {
  sessionStorage.removeItem('miniacs_token');
  localStorage.removeItem('token');
};

export async function request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const token = getStoredToken();
  const headers = new Headers(options.headers);
  if (!(options.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  if (token) headers.set('Authorization', `Bearer ${token}`);

  const response = await fetch(`${API_BASE}${endpoint}`, { ...options, headers });
  if (!response.ok) {
    if (response.status === 401) {
      clearStoredSession();
      if (window.location.pathname !== '/login') window.location.assign('/login');
    }
    const error = await response.json().catch(() => ({ error: `Request failed (${response.status})` }));
    throw new Error(error.error || `Request failed (${response.status})`);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export const api = {
  health: () => request<{ status: string }>('/health'),
  getDevices: (limit = 50, offset = 0) => request<DeviceListResponse>(`/devices?limit=${limit}&offset=${offset}`),
  getDevice: (serial: string) => request<Device>(`/device/${encodeURIComponent(serial)}`),
  getDeviceStats: () => request<DeviceStats>('/devices/stats'),
  getDeviceAnalytics: () => request<{ rxPower: Record<string, number>; temperature: Record<string, number>; uptime: Record<string, number>; accessType: Record<string, number>; lastInform: Record<string, number>; wifiStations: Record<string, number> }>('/devices/analytics'),
  getDeviceParameters: (serial: string) => request<DeviceParameter[]>(`/device/${encodeURIComponent(serial)}/parameters`),
  getDeviceTasks: (serial: string) => request<Task[]>(`/device/${encodeURIComponent(serial)}/tasks`),
  getParameterValues: (serial: string, parameters: string[]) => request<Task>(`/device/${encodeURIComponent(serial)}/get-parameters`, { method: 'POST', body: JSON.stringify({ parameters }) }),
  setParameterValues: (serial: string, parameters: Record<string, string>) => request<Task>(`/device/${encodeURIComponent(serial)}/set-parameters`, { method: 'POST', body: JSON.stringify({ parameters }) }),
  rebootDevice: (serial: string) => request<Task>(`/device/${encodeURIComponent(serial)}/reboot`, { method: 'POST' }),
  factoryResetDevice: (serial: string) => request<Task>(`/device/${encodeURIComponent(serial)}/factory-reset`, { method: 'POST' }),
  deleteDevice: (serial: string) => request<{ status: string }>(`/device/${encodeURIComponent(serial)}`, { method: 'DELETE' }),
  connectionRequest: (serial: string) => request<{ status: string; url: string; message: string }>(`/device/${encodeURIComponent(serial)}/connection-request`, { method: 'POST' }),
  downloadFirmware: (serial: string, firmwareId: number, fileType?: string) => request<Task>(`/device/${encodeURIComponent(serial)}/download-firmware`, { method: 'POST', body: JSON.stringify({ firmware_id: firmwareId, file_type: fileType }) }),

  getFirmwares: () => request<Firmware[]>('/firmwares'),
  uploadFirmware: (file: File, version: string, manufacturer?: string, productClass?: string, description?: string) => {
    const body = new FormData();
    body.append('file', file);
    body.append('version', version);
    if (manufacturer) body.append('manufacturer', manufacturer);
    if (productClass) body.append('product_class', productClass);
    if (description) body.append('description', description);
    return request<Firmware>('/firmwares', { method: 'POST', body });
  },
  deleteFirmware: (id: number) => request<{ status: string }>(`/firmwares/${id}`, { method: 'DELETE' }),

  getFaults: (filter: 'all' | 'active' | 'resolved' = 'active') => {
    const resolved = filter === 'all' ? '' : `&resolved=${filter === 'resolved'}`;
    return request<{ faults: Fault[]; total: number; limit: number; offset: number }>(`/faults?limit=100${resolved}`);
  },
  getFaultStats: () => request<{ total: number; active: number; resolved: number }>('/faults/stats'),
  resolveFault: (id: number) => request<{ status: string }>(`/faults/${id}/resolve`, { method: 'POST' }),
  deleteFault: (id: number) => request<{ status: string }>(`/faults/${id}`, { method: 'DELETE' }),

  getSettings: () => request<Record<string, string>>('/settings'),
  updateSettings: (settings: Record<string, string>) => request<{ status: string }>('/settings', { method: 'PUT', body: JSON.stringify(settings) }),
  getUsers: () => request<User[]>('/users'),
  createUser: (body: { username: string; password: string; role: 'full' | 'read' }) => request<User>('/users', { method: 'POST', body: JSON.stringify(body) }),
  updateUser: (id: number, body: Partial<{ username: string; password: string; role: 'full' | 'read' }>) => request<User>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteUser: (id: number) => request<{ status: string }>(`/users/${id}`, { method: 'DELETE' }),
  changePassword: (currentPassword: string, newPassword: string) => request<{ status: string }>('/auth/change-password', { method: 'POST', body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) }),
  getProvisioningRules: () => request<ProvisioningRule[]>('/provisioning'),
  createProvisioningRule: (body: Omit<ProvisioningRule, 'id'>) => request<ProvisioningRule>('/provisioning', { method: 'POST', body: JSON.stringify(body) }),
  updateProvisioningRule: (id: number, body: Omit<ProvisioningRule, 'id'>) => request<ProvisioningRule>(`/provisioning/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteProvisioningRule: (id: number) => request<{ status: string }>(`/provisioning/${id}`, { method: 'DELETE' }),
  toggleProvisioningRule: (id: number, enabled: boolean) => request<{ status: string }>(`/provisioning/${id}/toggle`, { method: 'POST', body: JSON.stringify({ enabled }) }),

  getSecurityOverview: () => request<{ total_users: number; full_access_admins: number; recorded_failures: number; jwt_configured: boolean; login_rate_limit_enabled: boolean; cors_restricted: boolean; audit_logging_enabled: boolean }>('/security/overview'),
  getAuditLogs: (limit = 100) => request<{ entries: AuditLog[]; total: number; limit: number; offset: number }>(`/audit-logs?limit=${limit}`),
  getBlockedDevices: () => request<BlockedDevice[]>('/blocked-devices'),
  blockDevice: (serialNumber: string, reason: string) => request<BlockedDevice>('/blocked-devices', { method: 'POST', body: JSON.stringify({ serial_number: serialNumber, reason }) }),
  unblockDevice: (serialNumber: string) => request<{ status: string }>(`/blocked-devices/${encodeURIComponent(serialNumber)}`, { method: 'DELETE' }),
};
