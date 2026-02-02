import type { Component } from 'solid-js';
import { createResource, Show, createMemo } from 'solid-js';
import { SolidApexCharts } from 'solid-apexcharts';
import { api } from '../lib/api';
import { useTheme } from '../lib/theme';
import { TrendingUp, Wifi, Thermometer, Radio, Clock, Activity } from 'lucide-solid';
import type { ApexOptions } from 'apexcharts';

const Dashboard: Component = () => {
  const [stats] = createResource(() => api.getDeviceStats());
  const [deviceList] = createResource(() => api.getDevices(100, 0));
  const [analytics] = createResource(() => api.getDeviceAnalytics());
  const { isDark } = useTheme();

  // ============================================
  // SECTION 1: Hero Stats (Total, Online, Offline)
  // ============================================
  const onlinePercent = createMemo(() => {
    const total = stats()?.total ?? 0;
    const online = stats()?.online ?? 0;
    return total > 0 ? Math.round((online / total) * 100) : 0;
  });

  // ============================================
  // SECTION 2: Last Inform - Horizontal Bar Chart
  // ============================================
  const lastInformBarOptions = createMemo<ApexOptions>(() => ({
    chart: {
      type: 'bar',
      background: 'transparent',
      toolbar: { show: false },
      animations: { enabled: true, speed: 400 },
    },
    plotOptions: {
      bar: {
        horizontal: true,
        borderRadius: 4,
        barHeight: '60%',
        distributed: true,
      }
    },
    colors: ['#10b981', '#84cc16', '#f59e0b', '#f97316', '#ef4444', '#7f1d1d'],
    dataLabels: {
      enabled: true,
      formatter: (val: number) => val > 0 ? String(val) : '',
      style: { fontSize: '11px', fontWeight: 600, colors: ['#fff'] },
      offsetX: 5,
    },
    xaxis: {
      labels: { show: false },
      axisBorder: { show: false },
      axisTicks: { show: false },
    },
    yaxis: {
      labels: { 
        style: { colors: isDark() ? '#a1a1aa' : '#52525b', fontSize: '11px' }
      },
    },
    grid: { show: false },
    legend: { show: false },
    tooltip: { enabled: false },
  }));

  const lastInformData = createMemo(() => {
    const data = analytics()?.lastInform ?? {};
    const labels = Object.keys(data);
    const series = Object.values(data) as number[];
    return { labels, series: [{ data: series }] };
  });

  // ============================================
  // SECTION 3: Device Health Metrics (Numbers)
  // ============================================
  const avgUptime = createMemo(() => {
    const data = analytics()?.uptime ?? {};
    const weights: Record<string, number> = { '<1d': 0.5, '1-7d': 4, '7-30d': 18, '>30d': 45 };
    let total = 0, count = 0;
    Object.entries(data).forEach(([k, v]) => {
      if (weights[k]) { total += weights[k] * (v as number); count += v as number; }
    });
    return count > 0 ? (total / count).toFixed(1) : '0';
  });

  const tempStatus = createMemo(() => {
    const data = analytics()?.temperature ?? {};
    const normal = (data['Normal'] as number) || 0;
    const warning = (data['Warning'] as number) || 0;
    const critical = (data['Critical'] as number) || 0;
    const total = normal + warning + critical;
    if (critical > 0) return { status: 'critical', color: 'text-rose-500', bg: 'bg-rose-500/10', count: critical, total };
    if (warning > 0) return { status: 'warning', color: 'text-amber-500', bg: 'bg-amber-500/10', count: warning, total };
    return { status: 'normal', color: 'text-emerald-500', bg: 'bg-emerald-500/10', count: normal, total };
  });

  const rxStatus = createMemo(() => {
    const data = analytics()?.rxPower ?? {};
    const good = (data['Excellent'] as number || 0) + (data['Good'] as number || 0);
    const warning = (data['Fair'] as number) || 0;
    const poor = (data['Poor'] as number) || 0;
    const total = good + warning + poor;
    if (poor > 0) return { status: 'poor', color: 'text-rose-500', bg: 'bg-rose-500/10', count: poor, total };
    if (warning > 0) return { status: 'fair', color: 'text-amber-500', bg: 'bg-amber-500/10', count: warning, total };
    return { status: 'good', color: 'text-emerald-500', bg: 'bg-emerald-500/10', count: good, total };
  });

  const wifiStats = createMemo(() => {
    const data = analytics()?.wifiStations ?? {};
    let total = 0;
    Object.values(data).forEach(v => total += v as number);
    return total;
  });

  // ============================================
  // SECTION 4: Categorical Data (Pie Charts)
  // ============================================
  const pieTheme = createMemo<Partial<ApexOptions>>(() => ({
    chart: { background: 'transparent', toolbar: { show: false } },
    theme: { mode: isDark() ? 'dark' : 'light' },
    stroke: { width: 2, colors: [isDark() ? '#18181b' : '#ffffff'] },
    legend: {
      position: 'bottom',
      fontSize: '11px',
      labels: { colors: isDark() ? '#a1a1aa' : '#52525b' },
    },
    dataLabels: { enabled: false },
    tooltip: { 
      theme: isDark() ? 'dark' : 'light',
      y: { formatter: (val: number) => `${val} devices` }
    }
  }));

  const accessTypeData = createMemo(() => {
    const data = analytics()?.accessType ?? {};
    return { labels: Object.keys(data), series: Object.values(data) as number[] };
  });

  const manufacturerData = createMemo(() => {
    const devices = deviceList()?.devices ?? [];
    const counts: Record<string, number> = {};
    devices.forEach(d => { counts[d.manufacturer || 'Unknown'] = (counts[d.manufacturer || 'Unknown'] || 0) + 1; });
    return { labels: Object.keys(counts), series: Object.values(counts) };
  });

  // ============================================
  // SECTION 5: Product Class (Horizontal Bar)
  // ============================================
  const productClassData = createMemo(() => {
    const devices = deviceList()?.devices ?? [];
    const counts: Record<string, number> = {};
    devices.forEach(d => {
      const pc = d.product_class || d.model_name || 'Unknown';
      counts[pc] = (counts[pc] || 0) + 1;
    });
    const sorted = Object.entries(counts).sort((a, b) => b[1] - a[1]).slice(0, 6);
    return { labels: sorted.map(e => e[0]), series: [{ data: sorted.map(e => e[1]) }] };
  });

  const productBarOptions = createMemo<ApexOptions>(() => ({
    chart: { type: 'bar', background: 'transparent', toolbar: { show: false } },
    plotOptions: {
      bar: { horizontal: true, borderRadius: 4, barHeight: '50%' }
    },
    colors: ['#14b8a6'],
    dataLabels: {
      enabled: true,
      formatter: (val: number) => String(val),
      style: { fontSize: '11px', colors: ['#fff'] },
    },
    xaxis: { labels: { show: false }, axisBorder: { show: false }, axisTicks: { show: false } },
    yaxis: { labels: { style: { colors: isDark() ? '#a1a1aa' : '#52525b', fontSize: '11px' } } },
    grid: { show: false },
    tooltip: { enabled: false },
  }));

  return (
    <div class="space-y-6">
      {/* ==================== ROW 1: Hero Stats ==================== */}
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* Total Devices - Large Card */}
        <div class="card p-6 lg:row-span-2">
          <div class="flex items-start justify-between mb-4">
            <div>
              <p class="text-sm font-medium text-muted mb-1">Total Devices</p>
              <p class="text-5xl font-bold text-primary font-mono tracking-tight">
                {stats()?.total ?? '-'}
              </p>
            </div>
            <div class="p-3 rounded-xl bg-teal-500/10">
              <Activity size={24} class="text-teal-500" />
            </div>
          </div>
          
          {/* Online/Offline Split */}
          <div class="mt-6 pt-4 border-t border-subtle">
            <div class="flex items-center gap-6">
              <div class="flex-1">
                <div class="flex items-center gap-2 mb-2">
                  <span class="w-2 h-2 rounded-full bg-emerald-500" />
                  <span class="text-xs text-muted">Online</span>
                </div>
                <p class="text-2xl font-bold text-emerald-500 font-mono">{stats()?.online ?? 0}</p>
              </div>
              <div class="w-px h-12 bg-subtle" />
              <div class="flex-1">
                <div class="flex items-center gap-2 mb-2">
                  <span class="w-2 h-2 rounded-full bg-rose-500" />
                  <span class="text-xs text-muted">Offline</span>
                </div>
                <p class="text-2xl font-bold text-rose-500 font-mono">{stats()?.offline ?? 0}</p>
              </div>
            </div>
            
            {/* Progress Bar */}
            <div class="mt-4">
              <div class="h-2 rounded-full bg-elevated overflow-hidden">
                <div 
                  class="h-full bg-gradient-to-r from-emerald-500 to-emerald-400 transition-all duration-500"
                  style={{ width: `${onlinePercent()}%` }}
                />
              </div>
              <p class="text-xs text-muted mt-2">{onlinePercent()}% online</p>
            </div>
          </div>
        </div>

        {/* Last Inform - Bar Chart */}
        <div class="card p-5 lg:col-span-2">
          <div class="flex items-center justify-between mb-3">
            <div>
              <h3 class="text-sm font-semibold text-primary">Last Inform</h3>
              <p class="text-xs text-muted">Time since last device contact</p>
            </div>
            <Clock size={18} class="text-muted" />
          </div>
          <Show
            when={lastInformData().series[0]?.data?.some((v: number) => v > 0)}
            fallback={<div class="h-32 flex items-center justify-center text-muted text-sm">No data</div>}
          >
            <SolidApexCharts
              type="bar"
              options={{ ...lastInformBarOptions(), xaxis: { ...lastInformBarOptions().xaxis, categories: lastInformData().labels } }}
              series={lastInformData().series}
              height={140}
            />
          </Show>
        </div>

        {/* Health Metrics Row */}
        <div class="lg:col-span-2 grid grid-cols-2 md:grid-cols-4 gap-3">
          {/* Avg Uptime */}
          <div class="card p-4">
            <div class="flex items-center gap-2 mb-2">
              <TrendingUp size={14} class="text-muted" />
              <span class="text-xs text-muted">Avg Uptime</span>
            </div>
            <p class="text-2xl font-bold text-primary font-mono">{avgUptime()}<span class="text-sm font-normal text-muted ml-1">days</span></p>
          </div>

          {/* Temperature Status */}
          <div class="card p-4">
            <div class="flex items-center gap-2 mb-2">
              <Thermometer size={14} class="text-muted" />
              <span class="text-xs text-muted">Temperature</span>
            </div>
            <div class="flex items-center gap-2">
              <span class={`px-2 py-0.5 rounded text-xs font-medium ${tempStatus().bg} ${tempStatus().color}`}>
                {tempStatus().status}
              </span>
              <span class="text-xs text-muted">{tempStatus().count}/{tempStatus().total}</span>
            </div>
          </div>

          {/* RX Power Status */}
          <div class="card p-4">
            <div class="flex items-center gap-2 mb-2">
              <Radio size={14} class="text-muted" />
              <span class="text-xs text-muted">RX Power</span>
            </div>
            <div class="flex items-center gap-2">
              <span class={`px-2 py-0.5 rounded text-xs font-medium ${rxStatus().bg} ${rxStatus().color}`}>
                {rxStatus().status}
              </span>
              <span class="text-xs text-muted">{rxStatus().count}/{rxStatus().total}</span>
            </div>
          </div>

          {/* WiFi Clients */}
          <div class="card p-4">
            <div class="flex items-center gap-2 mb-2">
              <Wifi size={14} class="text-muted" />
              <span class="text-xs text-muted">WiFi Clients</span>
            </div>
            <p class="text-2xl font-bold text-primary font-mono">{wifiStats()}</p>
          </div>
        </div>
      </div>

      {/* ==================== ROW 2: Categorical + Product Class ==================== */}
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {/* Access Type - Pie */}
        <div class="card p-5">
          <h3 class="text-sm font-semibold text-primary mb-1">Access Type</h3>
          <p class="text-xs text-muted mb-3">Connection technology distribution</p>
          <Show
            when={accessTypeData().series.some(v => v > 0)}
            fallback={<div class="h-40 flex items-center justify-center text-muted text-sm">No data</div>}
          >
            <SolidApexCharts
              type="donut"
              options={{ ...pieTheme(), labels: accessTypeData().labels, colors: ['#3b82f6', '#06b6d4', '#8b5cf6', '#6b7280'] }}
              series={accessTypeData().series}
              height={160}
            />
          </Show>
        </div>

        {/* Manufacturer - Pie */}
        <div class="card p-5">
          <h3 class="text-sm font-semibold text-primary mb-1">Manufacturer</h3>
          <p class="text-xs text-muted mb-3">Device vendor breakdown</p>
          <Show
            when={manufacturerData().series.some(v => v > 0)}
            fallback={<div class="h-40 flex items-center justify-center text-muted text-sm">No data</div>}
          >
            <SolidApexCharts
              type="donut"
              options={{ ...pieTheme(), labels: manufacturerData().labels, colors: ['#f59e0b', '#10b981', '#3b82f6', '#ef4444', '#8b5cf6'] }}
              series={manufacturerData().series}
              height={160}
            />
          </Show>
        </div>

        {/* Product Class - Horizontal Bar */}
        <div class="card p-5 md:col-span-2 lg:col-span-1">
          <h3 class="text-sm font-semibold text-primary mb-1">Product Class</h3>
          <p class="text-xs text-muted mb-3">Top device models</p>
          <Show
            when={productClassData().series[0]?.data?.length > 0}
            fallback={<div class="h-40 flex items-center justify-center text-muted text-sm">No data</div>}
          >
            <SolidApexCharts
              type="bar"
              options={{ ...productBarOptions(), xaxis: { ...productBarOptions().xaxis, categories: productClassData().labels } }}
              series={productClassData().series}
              height={160}
            />
          </Show>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
