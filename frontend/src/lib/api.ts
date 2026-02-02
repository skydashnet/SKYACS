const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:7547/api';

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
}

export interface DeviceStats {
  total: number;
  online: number;
  offline: number;
}

export interface DeviceListResponse {
  devices: Device[];
  total: number;
  limit: number;
  offset: number;
}

export interface DeviceParameter {
  id: number;
  device_id: number;
  name: string;
  value: string;
  updated_at: string;
}

export interface Task {
  id: number;
  device_id: number;
  type: string;
  payload: unknown;
  status: string;
  result: unknown;
  error_message?: string;
  created_at: string;
  sent_at?: string;
  completed_at?: string;
}

export interface Firmware {
  id: number;
  filename: string;
  version: string;
  manufacturer?: string;
  product_class?: string;
  file_size: number;
  file_path: string;
  checksum?: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

async function fetchAPI<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem('token');
  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options?.headers,
    },
  });

  if (!response.ok) {
    if (response.status === 401) {
      localStorage.removeItem('token');
      window.location.href = '/login';
      throw new Error('Session expired');
    }
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

export const api = {
  // Health check
  health: () => fetchAPI<{ status: string }>('/health'),

  // Devices
  getDevices: (limit = 50, offset = 0) =>
    fetchAPI<DeviceListResponse>(`/devices?limit=${limit}&offset=${offset}`),

  getDevice: (serial: string) =>
    fetchAPI<Device>(`/device/${serial}`),

  getDeviceStats: () =>
    fetchAPI<DeviceStats>('/devices/stats'),

  getDeviceAnalytics: () =>
    fetchAPI<{
      rxPower: Record<string, number>;
      temperature: Record<string, number>;
      uptime: Record<string, number>;
      accessType: Record<string, number>;
      lastInform: Record<string, number>;
      wifiStations: Record<string, number>;
    }>('/devices/analytics'),

  getDeviceParameters: (serial: string) =>
    fetchAPI<DeviceParameter[]>(`/device/${serial}/parameters`),

  getDeviceTasks: (serial: string) =>
    fetchAPI<Task[]>(`/device/${serial}/tasks`),

  // Device Actions
  getParameterValues: (serial: string, parameters: string[]) =>
    fetchAPI<Task>(`/device/${serial}/get-parameters`, {
      method: 'POST',
      body: JSON.stringify({ parameters }),
    }),

  setParameterValues: (serial: string, parameters: Record<string, string>) =>
    fetchAPI<Task>(`/device/${serial}/set-parameters`, {
      method: 'POST',
      body: JSON.stringify({ parameters }),
    }),

  rebootDevice: (serial: string) =>
    fetchAPI<Task>(`/device/${serial}/reboot`, { method: 'POST' }),

  factoryResetDevice: (serial: string) =>
    fetchAPI<Task>(`/device/${serial}/factory-reset`, { method: 'POST' }),

  deleteDevice: (serial: string) =>
    fetchAPI<{ status: string }>(`/device/${serial}`, { method: 'DELETE' }),

  connectionRequest: (serial: string) =>
    fetchAPI<{ status: string; url: string; message: string }>(`/device/${serial}/connection-request`, { method: 'POST' }),

  downloadFirmware: (serial: string, firmwareId: number, fileType?: string) =>
    fetchAPI<Task>(`/device/${serial}/download-firmware`, {
      method: 'POST',
      body: JSON.stringify({ firmware_id: firmwareId, file_type: fileType }),
    }),

  // Firmwares
  getFirmwares: () => fetchAPI<Firmware[]>('/firmwares'),

  uploadFirmware: async (file: File, version: string, manufacturer?: string, productClass?: string, description?: string): Promise<Firmware> => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('version', version);
    if (manufacturer) formData.append('manufacturer', manufacturer);
    if (productClass) formData.append('product_class', productClass);
    if (description) formData.append('description', description);

    const response = await fetch(`${API_BASE}/firmwares`, {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Unknown error' }));
      throw new Error(error.error || `HTTP ${response.status}`);
    }

    return response.json();
  },

  deleteFirmware: (id: number) =>
    fetchAPI<{ status: string }>(`/firmwares/${id}`, { method: 'DELETE' }),

  // Settings
  getSettings: () =>
    fetchAPI<Record<string, string>>('/settings'),

  updateSettings: (settings: Record<string, string>) =>
    fetchAPI<{ status: string }>('/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),
};

