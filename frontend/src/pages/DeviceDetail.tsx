import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For, createEffect, onMount, onCleanup } from 'solid-js';
import { useParams, A, useNavigate } from '@solidjs/router';
import { ArrowLeft, RefreshCw, RotateCcw, Trash2, Server, Network, Radio, Users, Zap, Edit, Save, X, HeartPulse, Send } from 'lucide-solid';
import { api } from '../lib/api';

const DeviceDetail: Component = () => {
  const params = useParams<{ serial: string }>();
  const navigate = useNavigate();
  const serial = () => params.serial || '';

  const [device, { refetch: refetchDevice }] = createResource(serial, api.getDevice);
  const [parameters, { refetch: refetchParams }] = createResource(serial, api.getDeviceParameters);
  const [tasks, { refetch: refetchTasks }] = createResource(serial, api.getDeviceTasks);

  const [actionLoading, setActionLoading] = createSignal<string | null>(null);
  const [message, setMessage] = createSignal<{ type: 'success' | 'error'; text: string } | null>(null);
  const [paramFilter, setParamFilter] = createSignal('');
  const [editingWifi, setEditingWifi] = createSignal<number | null>(null);
  const [wifiEdits, setWifiEdits] = createSignal<Record<string, string>>({});
  const [editingPPP, setEditingPPP] = createSignal<number | null>(null);
  const [pppEdits, setPPPEdits] = createSignal<Record<string, string>>({});
  const [autoRefresh] = createSignal(true);
  const [selectedParam, setSelectedParam] = createSignal<{ name: string; value: string } | null>(null);

  const autoFetchParameters = async () => {
    if (!device() || parameters()?.length) return;
    try {
      const allParams = [
        'InternetGatewayDevice.',
      ];
      await api.getParameterValues(serial(), allParams);
      showMessage('success', 'Parameter fetch task created. Tunggu beberapa detik lalu refresh.');
    } catch (err) {
      console.log('Auto-fetch skipped:', err);
    }
  };


  createEffect(() => {
    if (device() && !device.loading && (!parameters() || parameters()?.length === 0)) {
      setTimeout(autoFetchParameters, 1000);
    }
  });

  onMount(() => {
    const interval = setInterval(() => {
      if (autoRefresh()) {
        refetchDevice();
        refetchParams();
        refetchTasks();
      }
    }, 5000);
    onCleanup(() => clearInterval(interval));
  });

  // Dynamic page title
  createEffect(() => {
    const s = serial();
    document.title = s ? `${s} - miniACS` : 'miniACS';
    onCleanup(() => { document.title = 'miniACS'; });
  });

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 5000);
  };

  const handleReboot = async () => {
    if (!confirm('Reboot device ini?')) return;
    setActionLoading('reboot');
    try {
      await api.rebootDevice(serial());
      showMessage('success', 'Reboot task created.');
      refetchTasks();
    } catch (err) {
      showMessage('error', 'Gagal: ' + (err as Error).message);
    }
    setActionLoading(null);
  };

  const handleFactoryReset = async () => {
    if (!confirm('WARNING: Factory Reset akan menghapus semua konfigurasi!')) return;
    setActionLoading('factory-reset');
    try {
      await api.factoryResetDevice(serial());
      showMessage('success', 'Factory Reset task created.');
      refetchTasks();
    } catch (err) {
      showMessage('error', 'Gagal: ' + (err as Error).message);
    }
    setActionLoading(null);
  };

  const handleDelete = async () => {
    if (!confirm('Hapus device ini dari database?')) return;
    setActionLoading('delete');
    try {
      await api.deleteDevice(serial());
      navigate('/devices');
    } catch (err) {
      showMessage('error', 'Gagal: ' + (err as Error).message);
      setActionLoading(null);
    }
  };

  const handleSummon = async () => {
    setActionLoading('summon');
    try {
      const result = await api.connectionRequest(serial());
      showMessage('success', result.message);
      
      // Single task with safe paths
      await api.getParameterValues(serial(), [
        'InternetGatewayDevice.DeviceInfo.',
        'InternetGatewayDevice.LANDevice.1.Hosts.',
        'InternetGatewayDevice.LANDevice.1.WLANConfiguration.',
        'InternetGatewayDevice.WANDevice.1.WANConnectionDevice.',
      ]);
    } catch (err) {
      showMessage('error', 'Gagal: ' + (err as Error).message);
    }
    setActionLoading(null);
  };

  const handleEditWifi = (index: number, ssid: string, password: string) => {
    setEditingWifi(index);
    setWifiEdits({ ssid, password });
  };

  const handleCancelEditWifi = () => {
    setEditingWifi(null);
    setWifiEdits({});
  };

  const handleSaveWifi = async (index: number) => {
    const edits = wifiEdits();
    if (!edits.ssid && !edits.password) {
      handleCancelEditWifi();
      return;
    }

    setActionLoading(`wifi-${index}`);
    try {
      const params: Record<string, string> = {};
      const prefix = `InternetGatewayDevice.LANDevice.1.WLANConfiguration.${index}.`;
      
      if (edits.ssid) {
        params[prefix + 'SSID'] = edits.ssid;
      }
      if (edits.password) {
        params[prefix + 'PreSharedKey.1.PreSharedKey'] = edits.password;
      }

      await api.setParameterValues(serial(), params);
      showMessage('success', `WiFi SSID${index} update task created.`);
      refetchTasks();
      handleCancelEditWifi();
    } catch (err) {
      showMessage('error', 'Gagal: ' + (err as Error).message);
    }
    setActionLoading(null);
  };

  const handleSetWifiEnabled = async (index: number, enabled: boolean) => {
    if (!confirm(`${enabled ? 'Enable' : 'Disable'} SSID${index}?`)) return;
    
    setActionLoading(`wifi-enable-${index}`);
    try {
      const prefix = `InternetGatewayDevice.LANDevice.1.WLANConfiguration.${index}.`;
      await api.setParameterValues(serial(), {
        [prefix + 'Enable']: enabled ? '1' : '0'
      });
      showMessage('success', `SSID${index} ${enabled ? 'enabled' : 'disabled'} task created.`);
      refetchTasks();
    } catch (err) {
      showMessage('error', 'Gagal: ' + (err as Error).message);
    }
    setActionLoading(null);
  };

  // PPPoE Edit Handlers
  const handleEditPPP = (index: number, username: string, password: string) => {
    setEditingPPP(index);
    setPPPEdits({ username, password: password === '******' ? '' : password });
  };

  const handleCancelEditPPP = () => {
    setEditingPPP(null);
    setPPPEdits({});
  };

  const handleSavePPP = async (index: number) => {
    const edits = pppEdits();
    if (!edits.username && !edits.password) {
      handleCancelEditPPP();
      return;
    }

    setActionLoading(`ppp-${index}`);
    try {
      const params: Record<string, string> = {};
      const prefix = `InternetGatewayDevice.WANDevice.1.WANConnectionDevice.${index}.WANPPPConnection.1.`;
      
      if (edits.username) {
        params[prefix + 'Username'] = edits.username;
      }
      if (edits.password) {
        params[prefix + 'Password'] = edits.password;
      }

      await api.setParameterValues(serial(), params);
      showMessage('success', `PPPoE credentials untuk WAN${index} berhasil diubah.`);
      refetchTasks();
      handleCancelEditPPP();
    } catch (err) {
      showMessage('error', 'Gagal: ' + (err as Error).message);
    }
    setActionLoading(null);
  };

  const refreshAll = () => { refetchDevice(); refetchParams(); refetchTasks(); };

  const formatDate = (dateStr: string | null | undefined) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString('id-ID');
  };

  const formatUptime = (seconds: number | string) => {
    const secs = typeof seconds === 'string' ? parseInt(seconds) : seconds;
    if (isNaN(secs) || secs <= 0) return '-';
    const days = Math.floor(secs / 86400);
    const hours = Math.floor((secs % 86400) / 3600);
    const mins = Math.floor((secs % 3600) / 60);
    if (days > 0) return `${days}h ${hours}j ${mins}m`;
    if (hours > 0) return `${hours}j ${mins}m`;
    return `${mins}m`;
  };

  const getDeviceUptime = () => {
    const params = parameters() || [];
    const uptimeParam = params.find(p => p.name.endsWith('DeviceInfo.UpTime'));
    return uptimeParam?.value || null;
  };

  const getParamValue = (keywords: string[]) => {
    const params = parameters() || [];
    for (const kw of keywords) {
      const found = params.find(p => p.name.toLowerCase().includes(kw.toLowerCase()));
      if (found) return found.value;
    }
    return '-';
  };

  const getRxPower = () => {
    const val = getParamValue(['RXPower', 'RxPower', 'OpticalPower']);
    if (val === '-') return '-';
    const num = parseFloat(val);
    if (isNaN(num)) return val;
    if (num <= 0) return val;
    if (num > 0 && num < 10000) {
      const dBm = 10 * Math.log10(num / 10000);
      return dBm.toFixed(2);
    }
    if (Math.abs(num) > 100) return (num / 100).toFixed(2);
    return num.toFixed(2);
  };

  const getTemperature = () => {
    const val = getParamValue(['Temperature', 'Temp', 'OpticalTemperature']);
    if (val === '-') return '-';
    const num = parseFloat(val);
    if (isNaN(num)) return val;
    if (num > 1000) return (num / 256).toFixed(1);
    if (num > 100) return (num / 10).toFixed(1);
    return num.toFixed(1);
  };

  // Color-coded helpers
  const getTempColor = () => {
    const temp = parseFloat(getTemperature());
    if (isNaN(temp)) return 'text-muted';
    if (temp < 40) return 'text-emerald-400';
    if (temp < 55) return 'text-amber-400';
    return 'text-rose-400';
  };

  const getRxPowerColor = () => {
    const rx = parseFloat(getRxPower());
    if (isNaN(rx)) return 'text-muted';
    if (rx > -20) return 'text-emerald-400';
    if (rx > -25) return 'text-amber-400';
    return 'text-rose-400';
  };

  const getUptimeColor = () => {
    const uptime = getDeviceUptime();
    if (!uptime) return 'text-muted';
    const secs = parseInt(uptime);
    if (secs > 86400 * 7) return 'text-emerald-400'; // > 7 days
    if (secs > 86400) return 'text-teal-400'; // > 1 day
    return 'text-amber-400'; // < 1 day
  };

  const getWanConfigs = () => {
    const params = parameters() || [];
    const wanProfiles: Array<{
      index: number;
      name: string;
      status: string;
      vlan: string;
      username: string;
      password: string;
      ipAddress: string;
      service: string;
      nat: string;
      type: string;
      uptime: string;
      lan1: boolean;
      lan2: boolean;
      lan3: boolean;
      lan4: boolean;
      ssid1: boolean;
      ssid2: boolean;
      ssid3: boolean;
      ssid4: boolean; 
    }> = [];

    for (let i = 1; i <= 8; i++) {
      const wanPPP = params.filter(p => p.name.includes(`WANPPPConnection.${i}.`) || p.name.includes(`WANConnectionDevice.${i}.`));
      const wanIP = params.filter(p => p.name.includes(`WANIPConnection.${i}.`));
      const wanParams = [...wanPPP, ...wanIP];
      
      if (wanParams.length === 0) continue;

      const getName = (suffix: string) => wanParams.find(p => p.name.includes(suffix))?.value || '-';
      
      const isEnabled = (val: string) => val === '1' || val === 'true' || val.toLowerCase() === 'enable' || val.toLowerCase() === 'enabled' || val.toLowerCase() === 'yes';
      
      const detectPortBinding = (portNum: number, portType: 'lan' | 'ssid') => {
        const portPatterns = portType === 'lan' 
          ? [`Lan${portNum}Enable`, `LAN${portNum}Enable`, `Eth${portNum}Enable`, `ETH${portNum}`, `LAN${portNum}`, `Port${portNum}`]
          : [`SSID${portNum}Enable`, `Ssid${portNum}Enable`, `Wlan${portNum}Enable`, `WLAN${portNum}`, `SSID${portNum}`, `WiFi${portNum}`];
        
        for (const pattern of portPatterns) {
          const param = wanParams.find(p => p.name.includes(pattern));
          if (param && isEnabled(param.value)) return true;
        }
        
        const bindingParams = wanParams.filter(p => 
          p.name.toLowerCase().includes('binding') || 
          p.name.toLowerCase().includes('bindlist') ||
          p.name.toLowerCase().includes('servicelist') ||
          p.name.toLowerCase().includes('portmapping')
        );
        
        for (const bp of bindingParams) {
          const val = bp.value.toLowerCase();
          const searchTerms = portType === 'lan'
            ? [`lan${portNum}`, `eth${portNum}`, `port${portNum}`]
            : [`ssid${portNum}`, `wlan${portNum}`, `wifi${portNum}`];
          
          for (const term of searchTerms) {
            if (val.includes(term)) return true;
          }
        }
        
        return false;
      };

      const connStatus = getName('ConnectionStatus');
      if (connStatus === '-' && getName('Enable') !== '1') continue;

      const getFirstValid = (...suffixes: string[]) => {
        for (const s of suffixes) {
          const val = getName(s);
          if (val && val !== '-') return val;
        }
        return '-';
      };

      wanProfiles.push({
        index: i,
        name: getName('Name') !== '-' ? getName('Name') : `WAN${i}`,
        status: connStatus,
        vlan: getFirstValid('X_HW_VLAN', 'VLANID', 'VLANIDMark', 'X_CT_VLAN'),
        username: getName('Username'),
        password: getName('Password') || '******',
        ipAddress: getFirstValid('ExternalIPAddress', 'IPAddress'),
        service: getFirstValid('X_HW_SERVICELIST', 'X_HW_ServiceList', 'ServiceList', 'X_CT_ServiceList', 'X_CU_ServiceList'),
        nat: isEnabled(getName('NATEnabled')) ? 'Enabled' : 'Disabled',
        type: getName('ConnectionType') || (getName('Username') !== '-' ? 'PPPoE' : 'DHCP'),
        uptime: getName('Uptime'),
        lan1: detectPortBinding(1, 'lan'),
        lan2: detectPortBinding(2, 'lan'),
        lan3: detectPortBinding(3, 'lan'),
        lan4: detectPortBinding(4, 'lan'),
        ssid1: detectPortBinding(1, 'ssid'),
        ssid2: detectPortBinding(2, 'ssid'),
        ssid3: detectPortBinding(3, 'ssid'),
        ssid4: detectPortBinding(4, 'ssid'),
      });
    }
    return wanProfiles;
  };

  const getWlanConfigs = () => {
    const params = parameters() || [];
    const wlans: Array<{
      index: number;
      enabled: boolean;
      status: string;
      ssid: string;
      security: string;
      password: string;
      frequency: string;
      channel: string;
      maxBitrate: string;
    }> = [];
    
    for (let i = 1; i <= 8; i++) {
      const prefix = `InternetGatewayDevice.LANDevice.1.WLANConfiguration.${i}.`;
      const getVal = (suffix: string) => params.find(p => p.name === prefix + suffix)?.value;
      
      const enabled = getVal('Enable');
      const ssid = getVal('SSID');
      
      if (ssid || enabled) {
        wlans.push({
          index: i,
          enabled: enabled === '1' || enabled === 'true',
          status: getVal('Status') || (enabled === '1' ? 'Up' : 'Down'),
          ssid: ssid || '-',
          security: getVal('BeaconType') || getVal('WPAEncryptionModes') || '-',
          password: getVal('PreSharedKey.1.KeyPassphrase') || getVal('KeyPassphrase') || getVal('X_HW_WPAKey') || '******',
          frequency: getVal('OperatingFrequencyBand') || (i <= 4 ? '2.4GHz' : '5GHz'),
          channel: getVal('Channel') || 'Auto',
          maxBitrate: getVal('MaxBitRate') || getVal('X_HW_MaxBitRate') || '-',
        });
      }
    }
    return wlans;
  };

  const getHosts = () => {
    const params = parameters() || [];
    const hosts: Array<{ index: number; hostname: string; ip: string; mac: string; interface: string; rssi?: string; uptime?: string }> = [];
    
    // Parse dari Hosts.Host (LAN hosts)
    const hostIndices = [...new Set(params.filter(p => p.name.includes('Hosts.Host.')).map(p => {
      const match = p.name.match(/Host\.(\d+)\./);
      return match ? parseInt(match[1]) : 0;
    }))].filter(i => i > 0);
    
    // Helper function to get WiFi band from SSID index
    const getWifiBand = (ssidIdx: number): string => {
      const wlanPrefix = `InternetGatewayDevice.LANDevice.1.WLANConfiguration.${ssidIdx}.`;
      const channel = params.find(p => p.name === wlanPrefix + 'Channel')?.value;
      const standard = params.find(p => p.name === wlanPrefix + 'Standard')?.value;
      const freq = params.find(p => p.name === wlanPrefix + 'X_HW_FrequencyBand')?.value;
      
      // Check frequency band parameter directly
      if (freq) {
        if (freq.includes('5') || freq.includes('5GHz')) return '5GHz';
        if (freq.includes('2.4') || freq.includes('2.4GHz')) return '2.4GHz';
      }
      
      // Check by channel number
      if (channel) {
        const ch = parseInt(channel);
        if (ch >= 36 && ch <= 177) return '5GHz';
        if (ch >= 1 && ch <= 14) return '2.4GHz';
      }
      
      // Check by standard
      if (standard) {
        if (standard.includes('ac') || standard.includes('ax')) return '5GHz';
        if (standard.includes('n') || standard.includes('g') || standard.includes('b')) return '2.4GHz';
      }
      
      // Default based on SSID index (common pattern: 1=2.4GHz, 5+=5GHz)
      return ssidIdx >= 5 ? '5GHz' : '2.4GHz';
    };

    hostIndices.forEach(i => {
      const prefix = `InternetGatewayDevice.LANDevice.1.Hosts.Host.${i}.`;
      const getVal = (suffix: string) => params.find(p => p.name === prefix + suffix)?.value || '-';
      
      let interfaceType = 'LAN';
      const layer2 = getVal('Layer2Interface');
      const intfType = getVal('InterfaceType');
      
      if (layer2 && layer2 !== '-') {
        const wlanMatch = layer2.match(/WLANConfiguration\.(\d+)/);
        if (wlanMatch) {
          const ssidIdx = parseInt(wlanMatch[1]);
          interfaceType = `WiFi ${getWifiBand(ssidIdx)}`;
        } else if (layer2.includes('Ethernet') || layer2.includes('LANEthernet')) {
          interfaceType = 'Ethernet';
        }
      } else if (intfType && intfType !== '-') {
        if (intfType === 'Ethernet' || intfType === 'LAN') {
          interfaceType = 'Ethernet';
        } else if (intfType === 'WiFi' || intfType === 'WLAN' || intfType === '802.11') {
          interfaceType = 'WiFi';
        } else if (intfType !== 'Unknown') {
          interfaceType = intfType;
        }
      }
      
      hosts.push({
        index: i,
        hostname: getVal('HostName'),
        ip: getVal('IPAddress'),
        mac: getVal('MACAddress'),
        interface: interfaceType,
      });
    });

    // Parse dari WLAN AssociatedDevice (WiFi clients)
    for (let ssidIdx = 1; ssidIdx <= 8; ssidIdx++) {
      const assocIndices = [...new Set(params.filter(p => 
        p.name.includes(`WLANConfiguration.${ssidIdx}.AssociatedDevice.`)
      ).map(p => {
        const match = p.name.match(/AssociatedDevice\.(\d+)\./);
        return match ? parseInt(match[1]) : 0;
      }))].filter(i => i > 0);

      assocIndices.forEach(i => {
        const prefix = `InternetGatewayDevice.LANDevice.1.WLANConfiguration.${ssidIdx}.AssociatedDevice.${i}.`;
        const getVal = (suffix: string) => params.find(p => p.name === prefix + suffix)?.value || '-';
        
        const mac = getVal('AssociatedDeviceMACAddress');
        if (mac && mac !== '-') {
          // Cek apakah sudah ada di hosts (dari Hosts.Host)
          const exists = hosts.some(h => h.mac.toLowerCase() === mac.toLowerCase());
          if (!exists) {
            hosts.push({
              index: 100 + ssidIdx * 10 + i,
              hostname: getVal('X_HW_AssociatedDevicedescriptions') || '-',
              ip: getVal('AssociatedDeviceIPAddress'),
              mac: mac,
              interface: `WiFi ${getWifiBand(ssidIdx)}`,
              rssi: getVal('X_HW_RSSI'),
              uptime: getVal('X_HW_Uptime'),
            });
          }
        }
      });
    }
    return hosts;
  };

  const filteredParams = () => {
    const filter = paramFilter().toLowerCase();
    if (!filter) return parameters() || [];
    return (parameters() || []).filter(p => 
      p.name.toLowerCase().includes(filter) || p.value.toLowerCase().includes(filter)
    );
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'completed': return 'badge-success';
      case 'pending': return 'badge-warning';
      case 'sent': return 'bg-teal-500/15 text-teal-400';
      case 'failed': return 'badge-error';
      default: return 'bg-zinc-700 text-secondary';
    }
  };

  return (
    <div class="space-y-5">
      {/* Header - More prominent serial/status */}
      <div class="flex flex-col sm:flex-row sm:items-start gap-3 sm:gap-4">
        <div class="flex-1">
          <div class="flex items-center gap-3 mb-1">
            <A href="/devices" class="text-muted hover:text-secondary transition-fast">
              <ArrowLeft size={18} />
            </A>
            <Show when={device()}>
              <span class={`px-2.5 py-1 rounded-md text-xs font-medium ${device()?.online ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' : 'bg-rose-500/15 text-rose-400 border border-rose-500/30'}`}>
                {device()?.online ? 'Online' : 'Offline'}
              </span>
            </Show>
          </div>
          <Show when={device()}>
            <h1 class="text-xl sm:text-2xl font-bold text-primary font-mono tracking-tight">{device()?.serial_number}</h1>
            <p class="text-sm text-muted mt-0.5">{device()?.manufacturer} {device()?.product_class}</p>
          </Show>
        </div>
        <div class="flex gap-2">
          <button onClick={handleSummon} disabled={actionLoading() !== null} class="btn btn-primary text-xs sm:text-sm">
            <Zap size={14} />
            <span class="hidden sm:inline">{actionLoading() === 'summon' ? '...' : 'Summon'}</span>
          </button>
          <button onClick={refreshAll} class="btn btn-secondary text-xs sm:text-sm">
            <RefreshCw size={14} />
            <span class="hidden sm:inline">Refresh</span>
          </button>
        </div>
      </div>

      <Show when={message()}>
        <div class={`p-3 rounded-md text-sm ${message()?.type === 'success' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'}`}>
          {message()?.text}
        </div>
      </Show>

      <Show when={device()} fallback={
        <div class="card p-6">
          <div class="skeleton h-6 w-48 mb-4" />
          <div class="grid grid-cols-2 gap-4">
            <div class="skeleton h-4 w-32" />
            <div class="skeleton h-4 w-32" />
          </div>
        </div>
      }>
        {(d) => (
          <>
            {/* Row 1: ONT Info + Device Health + Actions */}
            <div class="grid grid-cols-1 lg:grid-cols-12 gap-4">
              {/* ONT Information Card */}
              <div class="card p-5 lg:col-span-5">
                <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
                  <Server size={14} />
                  ONT Information
                </h2>
                <div class="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
                  <div><span class="text-muted">Serial:</span> <span class="text-primary font-mono">{d().serial_number}</span></div>
                  <div><span class="text-muted">OUI:</span> <span class="text-primary">{d().oui}</span></div>
                  <div><span class="text-muted">Manufacturer:</span> <span class="text-primary">{d().manufacturer || '-'}</span></div>
                  <div><span class="text-muted">Product:</span> <span class="text-primary">{d().product_class || '-'}</span></div>
                  <div><span class="text-muted">Model:</span> <span class="text-primary">{d().model_name || getParamValue(['ModelName', 'X_HW_ModelName', 'DeviceInfo.ModelName']) || '-'}</span></div>
                  <div><span class="text-muted">HW Version:</span> <span class="text-primary">{d().hardware_version || '-'}</span></div>
                  <div><span class="text-muted">SW Version:</span> <span class="text-primary">{d().software_version || '-'}</span></div>
                  <div><span class="text-muted">IP Address:</span> <span class="text-primary font-mono">{d().ip_address || '-'}</span></div>
                </div>
              </div>

              {/* Device Health - Compact Horizontal */}
              <div class="card p-4 lg:col-span-4 border-l-2 border-teal-500">
                <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
                  <HeartPulse size={14} />
                  Device Health
                </h2>
                <div class="grid grid-cols-3 gap-3 text-center">
                  <div>
                    <div class={`text-lg font-bold font-mono ${getUptimeColor()}`}>
                      {getDeviceUptime() ? formatUptime(getDeviceUptime()!) : '-'}
                    </div>
                    <div class="text-[10px] text-muted uppercase mt-0.5">Uptime</div>
                  </div>
                  <div>
                    <div class={`text-lg font-bold font-mono ${getRxPowerColor()}`}>
                      {getRxPower()}
                    </div>
                    <div class="text-[10px] text-muted uppercase mt-0.5">RX dBm</div>
                  </div>
                  <div>
                    <div class={`text-lg font-bold font-mono ${getTempColor()}`}>
                      {getTemperature()}°
                    </div>
                    <div class="text-[10px] text-muted uppercase mt-0.5">Temp</div>
                  </div>
                </div>
              </div>
              {/* Actions Card */}
              <div class="card p-5 lg:col-span-3">
                <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
                  <Send size={14} />
                  Actions
                </h2>
                <div class="space-y-2">
                  <button onClick={handleReboot} disabled={actionLoading() !== null} class="btn btn-secondary w-full justify-start text-sm py-2">
                    <RotateCcw size={14} />
                    {actionLoading() === 'reboot' ? '...' : 'Reboot'}
                  </button>
                  <button onClick={handleFactoryReset} disabled={actionLoading() !== null} class="btn btn-danger w-full justify-start text-sm py-2">
                    <RotateCcw size={14} />
                    Reset
                  </button>
                  <button onClick={handleDelete} disabled={actionLoading() !== null} class="btn btn-danger w-full justify-start text-sm py-2">
                    <Trash2 size={14} />
                    Delete
                  </button>
                </div>
              </div>
            </div>

            {/* Row 2: WAN Information */}
            <div class="card p-5">
              <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
                <Network size={14} />
                WAN Connections
              </h2>
              <Show when={getWanConfigs().length > 0} fallback={
                <p class="text-muted text-sm">No WAN configuration data. Click Summon to fetch.</p>
              }>
                <div class="overflow-x-auto">
                  <table class="w-full text-sm">
                    <thead class="sticky top-0 bg-base z-10">
                      <tr class="border-b-2 border-subtle bg-base">
                        <th class="text-left px-3 py-2.5 font-semibold text-primary">Name</th>
                        <th class="text-left px-3 py-2.5 font-semibold text-primary">Status</th>
                        <th class="text-left px-3 py-2.5 font-semibold text-primary">Uptime</th>
                        <th class="text-left px-3 py-2.5 font-semibold text-primary">Type</th>
                        <th class="text-left px-3 py-2.5 font-semibold text-primary">VLAN</th>
                        <th class="text-left px-3 py-2.5 font-semibold text-primary">Username</th>
                        <th class="text-left px-3 py-2.5 font-semibold text-primary">IP Address</th>
                        <th class="text-left px-3 py-2.5 font-semibold text-primary">Service</th>
                        <th class="text-left px-3 py-2.5 font-semibold text-primary">NAT</th>
                        <th class="text-center px-2 py-2.5 font-semibold text-primary">L1</th>
                        <th class="text-center px-2 py-2.5 font-semibold text-primary">L2</th>
                        <th class="text-center px-2 py-2.5 font-semibold text-primary">L3</th>
                        <th class="text-center px-2 py-2.5 font-semibold text-primary">L4</th>
                        <th class="text-center px-2 py-2.5 font-semibold text-primary">S1</th>
                        <th class="text-center px-2 py-2.5 font-semibold text-primary">S2</th>
                        <th class="text-center px-2 py-2.5 font-semibold text-primary">S3</th>
                        <th class="text-center px-2 py-2.5 font-semibold text-primary">S4</th>
                        <th class="text-left px-3 py-2.5 font-semibold text-primary"></th>
                      </tr>
                    </thead>
                    <tbody>
                      <For each={getWanConfigs()}>
                        {(wan) => (
                          <tr class="border-t border-subtle hover:bg-elevated/30 transition-colors">
                            <td class="px-3 py-2.5 text-primary font-medium">{wan.name}</td>
                            <td class="px-3 py-2.5">
                              <span class={`badge ${wan.status === 'Connected' ? 'badge-success' : 'badge-error'}`}>{wan.status}</span>
                            </td>
                            <td class="px-3 py-2.5 text-secondary">{formatUptime(wan.uptime)}</td>
                            <td class="px-3 py-2.5 text-secondary">{wan.type}</td>
                            <td class="px-3 py-2.5 text-primary font-mono">{wan.vlan}</td>
                            <td class="px-3 py-2.5">
                              <Show when={editingPPP() === wan.index} fallback={
                                <span class="text-secondary font-mono">{wan.username}</span>
                              }>
                                <div class="space-y-1">
                                  <input
                                    type="text"
                                    value={pppEdits().username || ''}
                                    onInput={(e) => setPPPEdits({ ...pppEdits(), username: e.currentTarget.value })}
                                    class="w-28 px-2 py-1 text-sm border border-default rounded bg-elevated text-primary"
                                    placeholder="Username"
                                  />
                                  <input
                                    type="password"
                                    value={pppEdits().password || ''}
                                    onInput={(e) => setPPPEdits({ ...pppEdits(), password: e.currentTarget.value })}
                                    class="w-28 px-2 py-1 text-sm border border-default rounded bg-elevated text-primary"
                                    placeholder="Password"
                                  />
                                </div>
                              </Show>
                            </td>
                            <td class="px-3 py-2.5 text-primary font-mono">{wan.ipAddress}</td>
                            <td class="px-3 py-2.5 text-secondary">{wan.service}</td>
                            <td class="px-3 py-2.5 text-secondary">{wan.nat}</td>
                            <td class="px-2 py-2.5 text-center">{wan.lan1 ? <span class="text-emerald-400">Y</span> : <span class="text-muted">-</span>}</td>
                            <td class="px-2 py-2.5 text-center">{wan.lan2 ? <span class="text-emerald-400">Y</span> : <span class="text-muted">-</span>}</td>
                            <td class="px-2 py-2.5 text-center">{wan.lan3 ? <span class="text-emerald-400">Y</span> : <span class="text-muted">-</span>}</td>
                            <td class="px-2 py-2.5 text-center">{wan.lan4 ? <span class="text-emerald-400">Y</span> : <span class="text-muted">-</span>}</td>
                            <td class="px-2 py-2.5 text-center">{wan.ssid1 ? <span class="text-emerald-400">Y</span> : <span class="text-muted">-</span>}</td>
                            <td class="px-2 py-2.5 text-center">{wan.ssid2 ? <span class="text-emerald-400">Y</span> : <span class="text-muted">-</span>}</td>
                            <td class="px-2 py-2.5 text-center">{wan.ssid3 ? <span class="text-emerald-400">Y</span> : <span class="text-muted">-</span>}</td>
                            <td class="px-2 py-2.5 text-center">{wan.ssid4 ? <span class="text-emerald-400">Y</span> : <span class="text-muted">-</span>}</td>
                            <td class="px-2 py-2">
                              <Show when={wan.type.includes('PPP') || (wan.username && wan.username !== '-')} fallback={<span class="text-muted text-xs">-</span>}>
                                <Show when={editingPPP() === wan.index} fallback={
                                  <button
                                    onClick={() => handleEditPPP(wan.index, wan.username, wan.password)}
                                    disabled={actionLoading() !== null}
                                    class="p-1 rounded hover:bg-elevated text-teal-500"
                                    title="Edit PPPoE"
                                  >
                                    <Edit size={14} />
                                  </button>
                                }>
                                  <div class="flex gap-1">
                                    <button
                                      onClick={() => handleSavePPP(wan.index)}
                                      disabled={actionLoading() === `ppp-${wan.index}`}
                                      class="p-1 rounded hover:bg-elevated text-emerald-500"
                                      title="Save"
                                    >
                                      <Save size={14} />
                                    </button>
                                    <button
                                      onClick={handleCancelEditPPP}
                                      disabled={actionLoading() !== null}
                                      class="p-1 rounded hover:bg-elevated text-rose-500"
                                      title="Cancel"
                                    >
                                      <X size={14} />
                                    </button>
                                  </div>
                                </Show>
                              </Show>
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>
            </div>

            {/* Row 3: WiFi Information */}
            <div class="card p-5 border-l-2 border-emerald-500">
              <h2 class="text-xs font-medium text-muted mb-4 flex items-center gap-2">
                <Radio size={14} />
                WiFi Configuration
              </h2>
              <Show when={getWlanConfigs().length > 0} fallback={
                <p class="text-muted text-sm">No WiFi configuration data. Click Summon to fetch.</p>
              }>
                <div class="overflow-x-auto">
                  <table class="w-full text-xs">
                    <thead class="sticky top-0 bg-base z-10">
                      <tr class="border-b border-subtle bg-base">
                        <th class="text-left px-2 py-2 font-medium text-secondary">Index</th>
                        <th class="text-left px-2 py-2 font-medium text-secondary">On</th>
                        <th class="text-left px-2 py-2 font-medium text-secondary">Status</th>
                        <th class="text-left px-2 py-2 font-medium text-secondary">SSID Name</th>
                        <th class="text-left px-2 py-2 font-medium text-secondary">Security</th>
                        <th class="text-left px-2 py-2 font-medium text-secondary">Band</th>
                        <th class="text-left px-2 py-2 font-medium text-secondary">Ch</th>
                        <th class="text-left px-2 py-2 font-medium text-secondary">Max Rate</th>
                        <th class="text-left px-2 py-2 font-medium text-secondary">Password</th>
                        <th class="text-left px-2 py-2 font-medium text-secondary"></th>
                      </tr>
                    </thead>
                    <tbody>
                      <For each={getWlanConfigs()}>
                        {(wlan, i) => (
                          <tr class={`border-t border-subtle/30 hover:bg-elevated/50 transition-colors ${i() % 2 === 1 ? 'bg-elevated/20' : ''}`}>
                            <td class="px-3 py-2 text-secondary">SSID{wlan.index}</td>
                            <td class="px-3 py-2">
                              <button 
                                onClick={() => handleSetWifiEnabled(wlan.index, !wlan.enabled)}
                                disabled={actionLoading() !== null}
                                class={`badge cursor-pointer ${wlan.enabled ? 'badge-success' : 'badge-error'}`}
                              >
                                {wlan.enabled ? 'Yes' : 'No'}
                              </button>
                            </td>
                            <td class="px-3 py-2 text-secondary">{wlan.status}</td>
                            <td class="px-3 py-2">
                              <Show when={editingWifi() === wlan.index} fallback={
                                <span class="text-primary font-medium">{wlan.ssid}</span>
                              }>
                                <input
                                  type="text"
                                  value={wifiEdits().ssid || ''}
                                  onInput={(e) => setWifiEdits({ ...wifiEdits(), ssid: e.currentTarget.value })}
                                  class="input py-1 px-2 text-sm w-32"
                                  placeholder="SSID Name"
                                />
                              </Show>
                            </td>
                            <td class="px-3 py-2 text-secondary">{wlan.security}</td>
                            <td class="px-3 py-2 text-secondary">{wlan.frequency}</td>
                            <td class="px-3 py-2 text-secondary">{wlan.channel}</td>
                            <td class="px-3 py-2 text-secondary">{wlan.maxBitrate}</td>
                            <td class="px-3 py-2">
                              <Show when={editingWifi() === wlan.index} fallback={
                                <span class="text-muted font-mono text-xs">{wlan.password}</span>
                              }>
                                <input
                                  type="text"
                                  value={wifiEdits().password || ''}
                                  onInput={(e) => setWifiEdits({ ...wifiEdits(), password: e.currentTarget.value })}
                                  class="input py-1 px-2 text-sm w-28"
                                  placeholder="Password"
                                />
                              </Show>
                            </td>
                            <td class="px-3 py-2">
                              <Show when={editingWifi() === wlan.index} fallback={
                                <button 
                                  onClick={() => handleEditWifi(wlan.index, wlan.ssid, wlan.password)}
                                  class="text-teal-400 hover:text-teal-300 flex items-center gap-1"
                                >
                                  <Edit size={12} /> Edit
                                </button>
                              }>
                                <div class="flex items-center gap-2">
                                  <button 
                                    onClick={() => handleSaveWifi(wlan.index)}
                                    disabled={actionLoading() === `wifi-${wlan.index}`}
                                    class="text-emerald-400 hover:text-emerald-300"
                                  >
                                    <Save size={14} />
                                  </button>
                                  <button 
                                    onClick={handleCancelEditWifi}
                                    class="text-secondary hover:text-secondary"
                                  >
                                    <X size={14} />
                                  </button>
                                </div>
                              </Show>
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>
            </div>

            {/* Row 4: Connected Hosts */}
            <div class="card p-5">
              <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
                <Users size={14} />
                Connected Hosts ({getHosts().length})
              </h2>
              <Show when={getHosts().length > 0} fallback={
                <p class="text-muted text-sm">No connected hosts data. Click Summon to fetch.</p>
              }>
                <div class="overflow-x-auto">
                  <table class="w-full text-sm">
                    <thead class="sticky top-0 bg-base z-10">
                      <tr class="border-b border-subtle bg-base">
                        <th class="text-left px-3 py-2 text-xs font-medium text-muted uppercase">Hostname</th>
                        <th class="text-left px-3 py-2 text-xs font-medium text-muted uppercase">IP Address</th>
                        <th class="text-left px-3 py-2 text-xs font-medium text-muted uppercase">MAC Address</th>
                        <th class="text-left px-3 py-2 text-xs font-medium text-muted uppercase">Interface</th>
                        <th class="text-left px-3 py-2 text-xs font-medium text-muted uppercase">Signal</th>
                        <th class="text-left px-3 py-2 text-xs font-medium text-muted uppercase">Uptime</th>
                      </tr>
                    </thead>
                    <tbody>
                      <For each={getHosts()}>
                        {(host) => (
                          <tr class="border-t border-subtle/50 hover:bg-elevated/30">
                            <td class="px-3 py-2 text-primary">{host.hostname}</td>
                            <td class="px-3 py-2 text-primary font-mono">{host.ip}</td>
                            <td class="px-3 py-2 text-secondary font-mono text-xs">{host.mac}</td>
                            <td class="px-3 py-2">
                              <span class={`px-2 py-0.5 rounded text-xs ${
                                host.interface.startsWith('WiFi') || host.interface.startsWith('SSID') 
                                  ? 'bg-teal-500/20 text-teal-400' 
                                  : host.interface === 'Ethernet' 
                                    ? 'bg-blue-500/20 text-blue-400'
                                    : 'bg-zinc-700 text-secondary'
                              }`}>
                                {host.interface}
                              </span>
                            </td>
                            <td class="px-3 py-2 text-secondary">
                              {host.rssi && host.rssi !== '-' ? `${host.rssi} dBm` : '-'}
                            </td>
                            <td class="px-3 py-2 text-secondary text-xs">
                              {host.uptime ? formatUptime(host.uptime) : '-'}
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>
            </div>

            {/* Row 5: Task History */}
            <div class="card overflow-hidden">
              <div class="p-5 border-b border-subtle">
                <h2 class="text-sm font-medium text-secondary">Task History ({tasks()?.length || 0})</h2>
              </div>
              <Show when={(tasks()?.length || 0) > 0} fallback={
                <div class="p-6 text-center">
                  <p class="text-muted text-sm">No tasks yet</p>
                </div>
              }>
                <div class="max-h-48 overflow-y-auto">
                  <table class="w-full text-sm">
                    <thead class="bg-base sticky top-0 z-10">
                      <tr class="bg-base">
                        <th class="text-left px-4 py-2 text-xs font-medium text-muted uppercase">Type</th>
                        <th class="text-left px-4 py-2 text-xs font-medium text-muted uppercase">Status</th>
                        <th class="text-left px-4 py-2 text-xs font-medium text-muted uppercase">Created</th>
                        <th class="text-left px-4 py-2 text-xs font-medium text-muted uppercase">Error</th>
                      </tr>
                    </thead>
                    <tbody>
                      <For each={tasks()}>
                        {(task) => (
                          <tr class="border-t border-subtle/50 hover:bg-elevated/30">
                            <td class="px-4 py-2 text-primary">{task.type}</td>
                            <td class="px-4 py-2"><span class={`badge ${getStatusBadge(task.status)}`}>{task.status}</span></td>
                            <td class="px-4 py-2 text-muted text-xs">{formatDate(task.created_at)}</td>
                            <td class="px-4 py-2 text-rose-400 text-xs">{task.error_message || '-'}</td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>
            </div>

            {/* Row 6: All Parameters */}
            <div class="card overflow-hidden">
              <div class="p-5 border-b border-subtle flex items-center justify-between">
                <h2 class="text-sm font-medium text-secondary">All Parameters ({filteredParams().length})</h2>
                <input
                  type="text"
                  value={paramFilter()}
                  onInput={(e) => setParamFilter(e.currentTarget.value)}
                  placeholder="Filter..."
                  class="input w-64 py-1.5 text-sm"
                />
              </div>
              <Show when={filteredParams().length > 0} fallback={
                <div class="p-8 text-center">
                  <p class="text-muted text-sm">No parameters loaded yet. Click Summon to fetch.</p>
                </div>
              }>
                <div class="max-h-96 overflow-auto">
                  <table class="w-full text-sm table-fixed">
                    <thead class="bg-base sticky top-0 z-10">
                      <tr class="bg-base">
                        <th class="text-left px-4 py-2 text-xs font-medium text-muted uppercase w-3/5">Name</th>
                        <th class="text-left px-4 py-2 text-xs font-medium text-muted uppercase w-2/5">Value</th>
                      </tr>
                    </thead>
                    <tbody>
                      <For each={filteredParams()}>
                        {(param) => (
                          <tr 
                            class="border-t border-subtle/50 hover:bg-elevated/30 cursor-pointer"
                            onClick={() => setSelectedParam(param)}
                          >
                            <td class="px-4 py-2 text-secondary font-mono text-xs truncate" title={param.name}>{param.name}</td>
                            <td class="px-4 py-2 text-primary text-xs truncate max-w-xs" title="Click to view full">
                              {param.value.length > 100 ? param.value.slice(0, 100) + '...' : param.value}
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>
            </div>
          </>
        )}
      </Show>

      {/* Parameter Detail Modal */}
      <Show when={selectedParam()}>
        <div class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={() => setSelectedParam(null)}>
          <div class="card p-5 w-full max-w-3xl max-h-[80vh] flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div class="flex items-center justify-between mb-4">
              <h3 class="text-sm font-medium text-secondary">Parameter Detail</h3>
              <button onClick={() => setSelectedParam(null)} class="text-muted hover:text-secondary">
                <X size={18} />
              </button>
            </div>
            <div class="mb-3">
              <label class="text-xs text-muted">Name</label>
              <p class="text-teal-400 font-mono text-sm break-all">{selectedParam()?.name}</p>
            </div>
            <div class="flex-1 overflow-auto">
              <label class="text-xs text-muted">Value</label>
              <pre class="mt-1 p-3 bg-elevated/50 rounded-lg text-primary text-sm font-mono whitespace-pre-wrap break-all overflow-auto max-h-96">{selectedParam()?.value}</pre>
            </div>
            <div class="mt-4 flex justify-end">
              <button onClick={() => setSelectedParam(null)} class="btn btn-secondary">
                Tutup
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};

export default DeviceDetail;
