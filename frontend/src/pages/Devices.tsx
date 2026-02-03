import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For, createMemo, createEffect, onMount } from 'solid-js';
import { A } from '@solidjs/router';
import { RefreshCw, ChevronLeft, ChevronRight, Router as RouterIcon, Settings2, X, Eye, EyeOff, GripVertical, Search, Send } from 'lucide-solid';
import { api } from '../lib/api';

interface ColumnConfig {
  id: string;
  label: string;
  visible: boolean;
  order: number;
}

const defaultColumns: ColumnConfig[] = [
  { id: 'serial_number', label: 'Serial Number', visible: true, order: 0 },
  { id: 'manufacturer', label: 'Manufacturer', visible: true, order: 1 },
  { id: 'model', label: 'Model', visible: true, order: 2 },
  { id: 'product_class', label: 'Product Class', visible: false, order: 3 },
  { id: 'ip_address', label: 'IP Address', visible: true, order: 4 },
  { id: 'rx_power', label: 'RX Power', visible: true, order: 5 },
  { id: 'ppp_username', label: 'PPP Username', visible: false, order: 6 },
  { id: 'status', label: 'Status', visible: true, order: 7 },
  { id: 'last_inform', label: 'Last Inform', visible: true, order: 8 },
];

const STORAGE_KEY = 'miniacs_device_columns';

const TableSkeleton: Component<{ cols: number }> = (props) => (
  <tr>
    <For each={Array(props.cols).fill(0)}>
      {() => <td class="px-4 py-3"><div class="skeleton h-4 w-24" /></td>}
    </For>
  </tr>
);

const Devices: Component = () => {
  const [page, setPage] = createSignal(0);
  const [showColumnSettings, setShowColumnSettings] = createSignal(false);
  const [activeFilter, setActiveFilter] = createSignal<string | null>(null);
  const [filters, setFilters] = createSignal<Record<string, string>>({});
  const [columns, setColumns] = createSignal<ColumnConfig[]>([]);
  const [summoningAll, setSummoningAll] = createSignal(false);
  const limit = 20;

  onMount(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      try {
        const parsed = JSON.parse(saved);
        const merged = defaultColumns.map(dc => {
          const saved = parsed.find((p: ColumnConfig) => p.id === dc.id);
          return saved ? { ...dc, visible: saved.visible, order: saved.order } : dc;
        });
        setColumns(merged.sort((a, b) => a.order - b.order));
      } catch {
        setColumns([...defaultColumns]);
      }
    } else {
      setColumns([...defaultColumns]);
    }
  });

  createEffect(() => {
    const cols = columns();
    if (cols.length > 0) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(cols));
    }
  });

  const [deviceList, { refetch }] = createResource(
    () => ({ page: page(), filters: filters() }),
    (params) => api.getDevices(limit, params.page * limit)
  );

  const totalPages = () => Math.ceil((deviceList()?.total ?? 0) / limit);

  const visibleColumns = createMemo(() => 
    columns().filter(c => c.visible).sort((a, b) => a.order - b.order)
  );

  const formatDate = (dateStr: string | null) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString('id-ID');
  };

  const toggleColumn = (id: string) => {
    setColumns(cols => cols.map(c => c.id === id ? { ...c, visible: !c.visible } : c));
  };

  const [draggedCol, setDraggedCol] = createSignal<string | null>(null);

  const handleDragStart = (e: DragEvent, id: string) => {
    setDraggedCol(id);
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', id);
    }
  };

  const handleDragOver = (e: DragEvent) => {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
  };

  const handleDrop = (e: DragEvent, targetId: string) => {
    e.preventDefault();
    const sourceId = draggedCol();
    if (!sourceId || sourceId === targetId) return;
    
    setColumns(cols => {
      const sorted = [...cols].sort((a, b) => a.order - b.order);
      const sourceIdx = sorted.findIndex(c => c.id === sourceId);
      const targetIdx = sorted.findIndex(c => c.id === targetId);
      if (sourceIdx === -1 || targetIdx === -1) return cols;
      
      const [removed] = sorted.splice(sourceIdx, 1);
      sorted.splice(targetIdx, 0, removed);
      return sorted.map((c, i) => ({ ...c, order: i }));
    });
    setDraggedCol(null);
  };

  const getRxPower = (device: any) => {
    const params = device.parameters || [];
    const rxParam = params.find((p: any) => 
      p.name.includes('RXPower') || 
      p.name.includes('OpticalReceivedPower') || 
      p.name.includes('TransceiverRxPower') ||
      p.name.includes('ReceivedPower') ||
      p.name.includes('X_HW_OpticalRxPower')
    );
    if (!rxParam) return { value: '-', color: 'text-muted' };
    const num = parseFloat(rxParam.value);
    if (isNaN(num)) return { value: rxParam.value, color: 'text-muted' };
    
    let val: number;
    if (num <= 0) {
      val = num;
    } else if (num > 0 && num < 10000) {
      val = 10 * Math.log10(num / 10000);
    } else if (Math.abs(num) > 100) {
      val = num / 100;
    } else {
      val = num;
    }
    
    const color = val > -20 ? 'text-emerald-400' : val > -25 ? 'text-amber-400' : 'text-rose-400';
    return { value: val.toFixed(2), color };
  };

  const getPppUsername = (device: any) => {
    const param = device.parameters?.find((p: any) => 
      p.name.includes('WANPPPConnection') && p.name.includes('Username')
    );
    return param?.value || '-';
  };

  const getCellValue = (device: any, columnId: string) => {
    switch (columnId) {
      case 'serial_number':
        return (
          <A href={`/device/${device.serial_number}`} class="text-teal-500 hover:text-teal-400 font-mono text-sm transition-fast">
            {device.serial_number}
          </A>
        );
      case 'manufacturer':
        return <span class="text-secondary text-sm">{device.manufacturer || '-'}</span>;
      case 'model':
        return <span class="text-secondary text-sm">{device.model_name || device.product_class || '-'}</span>;
      case 'product_class':
        return <span class="text-secondary text-sm">{device.product_class || '-'}</span>;
      case 'ip_address':
        return <span class="text-muted font-mono text-sm">{device.ip_address || '-'}</span>;
      case 'rx_power':
        const rx = getRxPower(device);
        return <span class={`font-mono text-sm ${rx.color}`}>{rx.value}</span>;
      case 'ppp_username':
        return <span class="text-secondary text-sm font-mono">{getPppUsername(device)}</span>;
      case 'status':
        return (
          <span class={`badge ${device.online ? 'badge-success' : 'badge-error'}`}>
            {device.online ? 'Online' : 'Offline'}
          </span>
        );
      case 'last_inform':
        return <span class="text-muted text-xs">{formatDate(device.last_inform)}</span>;
      default:
        return '-';
    }
  };

  const filteredDevices = createMemo(() => {
    const devices = deviceList()?.devices || [];
    const f = filters();
    if (Object.keys(f).length === 0) return devices;
    return devices.filter(d => {
      for (const [key, val] of Object.entries(f)) {
        if (!val) continue;
        const search = val.toLowerCase();
        switch (key) {
          case 'serial_number':
            if (!d.serial_number?.toLowerCase().includes(search)) return false;
            break;
          case 'manufacturer':
            if (!d.manufacturer?.toLowerCase().includes(search)) return false;
            break;
          case 'model':
            if (!(d.model_name?.toLowerCase().includes(search) || d.product_class?.toLowerCase().includes(search))) return false;
            break;
          case 'ip_address':
            if (!d.ip_address?.toLowerCase().includes(search)) return false;
            break;
          case 'status':
            const online = search === 'online' || search === 'on' || search === '1';
            const offline = search === 'offline' || search === 'off' || search === '0';
            if (online && !d.online) return false;
            if (offline && d.online) return false;
            break;
        }
      }
      return true;
    });
  });

  return (
    <div class="space-y-5">
      <div class="flex items-center justify-between gap-3 flex-wrap">
        <h1 class="text-xl font-semibold text-primary">List All Devices</h1>
        <div class="flex gap-2">
          <Show when={Object.values(filters()).some(v => v)}>
            <button onClick={() => setFilters({})} class="btn btn-secondary text-xs py-1 px-2">
              Clear Filters
            </button>
          </Show>
          <button onClick={() => setShowColumnSettings(!showColumnSettings())}
            class={`btn btn-secondary ${showColumnSettings() ? 'bg-teal-500/20 text-teal-400' : ''}`}
          >
            <Settings2 size={14} />
            <span class="hidden sm:inline">Columns</span>
          </button>
          <button 
            onClick={async () => {
              const list = deviceList()?.devices;
              if (!list || summoningAll()) return;
              setSummoningAll(true);
              const results = await Promise.allSettled(
                list.map((d: any) => api.connectionRequest(d.serial_number))
              );
              const successCount = results.filter((r: PromiseSettledResult<any>) => r.status === 'fulfilled').length;
              alert(`Summon selesai: ${successCount}/${list.length} berhasil`);
              setSummoningAll(false);
            }}
            disabled={summoningAll()}
            class="btn btn-secondary"
          >
            <Send size={14} class={summoningAll() ? 'animate-pulse' : ''} />
            <span class="hidden sm:inline">{summoningAll() ? 'Summoning...' : 'Summon All'}</span>
          </button>
          <button onClick={() => refetch()} class="btn btn-secondary">
            <RefreshCw size={14} />
            <span class="hidden sm:inline">Refresh</span>
          </button>
        </div>
      </div>

      {/* Column Settings Panel */}
      <Show when={showColumnSettings()}>
        <div class="card p-4">
          <div class="flex items-center justify-between mb-3">
            <h3 class="text-sm font-medium text-secondary">Manage Columns</h3>
            <button onClick={() => setShowColumnSettings(false)} class="text-muted hover:text-primary">
              <X size={16} />
            </button>
          </div>
          <div class="flex flex-wrap gap-2">
            <For each={columns().sort((a, b) => a.order - b.order)}>
              {(col) => (
                <div 
                  draggable={true}
                  onDragStart={(e) => handleDragStart(e, col.id)}
                  onDragOver={handleDragOver}
                  onDrop={(e) => handleDrop(e, col.id)}
                  class={`flex items-center gap-2 px-3 py-2 rounded-lg bg-elevated cursor-grab active:cursor-grabbing transition-all ${draggedCol() === col.id ? 'opacity-50 scale-95' : 'hover:bg-elevated/80'}`}
                >
                  <GripVertical size={12} class="text-muted" />
                  <button
                    onClick={() => toggleColumn(col.id)}
                    class={col.visible ? 'text-teal-500' : 'text-muted'}
                  >
                    {col.visible ? <Eye size={14} /> : <EyeOff size={14} />}
                  </button>
                  <span class={`text-sm ${col.visible ? 'text-primary' : 'text-muted'}`}>
                    {col.label}
                  </span>
                </div>
              )}
            </For>
          </div>
        </div>
      </Show>

      <div class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="border-b border-subtle">
                <For each={visibleColumns()}>
                  {(col) => {
                    const isFilterable = ['serial_number', 'manufacturer', 'model', 'ip_address', 'status', 'ppp_username'].includes(col.id);
                    const isActive = activeFilter() === col.id;
                    const hasFilter = !!filters()[col.id];
                    
                    return (
                      <th class="px-4 py-2 text-left whitespace-nowrap">
                        <Show when={isActive} fallback={
                          <button
                            onClick={() => isFilterable && setActiveFilter(col.id)}
                            class={`flex items-center gap-1 text-xs font-medium uppercase tracking-wide transition-colors ${
                              hasFilter ? 'text-teal-400' : 'text-muted'
                            } ${isFilterable ? 'hover:text-primary cursor-pointer' : 'cursor-default'}`}
                          >
                            {col.label}
                            <Show when={isFilterable}>
                              <Search size={10} class={hasFilter ? 'text-teal-400' : 'opacity-50'} />
                            </Show>
                          </button>
                        }>
                          <div class="flex items-center gap-1">
                            <Show when={col.id === 'status'} fallback={
                              <input
                                type="text"
                                value={filters()[col.id] || ''}
                                onInput={(e) => setFilters(f => ({ ...f, [col.id]: e.currentTarget.value }))}
                                onKeyDown={(e) => e.key === 'Escape' && setActiveFilter(null)}
                                onBlur={() => setTimeout(() => setActiveFilter(null), 150)}
                                class="input input-sm w-24 text-xs"
                                placeholder={`Filter...`}
                                autofocus
                              />
                            }>
                              <select
                                value={filters().status || ''}
                                onChange={(e) => {
                                  setFilters(f => ({ ...f, status: e.currentTarget.value }));
                                  setActiveFilter(null);
                                }}
                                onBlur={() => setTimeout(() => setActiveFilter(null), 150)}
                                class="input input-sm w-20 text-xs"
                                autofocus
                              >
                                <option value="">All</option>
                                <option value="online">Online</option>
                                <option value="offline">Offline</option>
                              </select>
                            </Show>
                            <button onClick={() => setActiveFilter(null)} class="text-muted hover:text-primary">
                              <X size={12} />
                            </button>
                          </div>
                        </Show>
                      </th>
                    );
                  }}
                </For>
              </tr>
            </thead>
            <tbody>
              <Show
                when={!deviceList.loading}
                fallback={<><TableSkeleton cols={visibleColumns().length} /><TableSkeleton cols={visibleColumns().length} /><TableSkeleton cols={visibleColumns().length} /></>}
              >
                <Show
                  when={filteredDevices().length}
                  fallback={
                    <tr>
                      <td colspan={visibleColumns().length} class="px-4 py-12 text-center">
                        <RouterIcon size={32} class="mx-auto text-muted mb-2 opacity-50" />
                        <p class="text-muted text-sm">No devices found</p>
                        <p class="text-muted opacity-50 text-xs mt-1">Devices will appear after connecting to the ACS</p>
                      </td>
                    </tr>
                  }
                >
                  <For each={filteredDevices()}>
                    {(device) => (
                      <tr class="border-t border-subtle hover:bg-elevated transition-fast">
                        <For each={visibleColumns()}>
                          {(col) => (
                            <td class="px-4 py-3 whitespace-nowrap">
                              {getCellValue(device, col.id)}
                            </td>
                          )}
                        </For>
                      </tr>
                    )}
                  </For>
                </Show>
              </Show>
            </tbody>
          </table>
        </div>
      </div>

      <Show when={totalPages() > 1}>
        <div class="flex items-center justify-between text-sm">
          <p class="text-muted">
            Showing {page() * limit + 1} - {Math.min((page() + 1) * limit, deviceList()?.total ?? 0)} of {deviceList()?.total ?? 0}
          </p>
          <div class="flex gap-2">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page() === 0}
              class="btn btn-secondary py-1.5 px-3"
            >
              <ChevronLeft size={14} />
              Previous
            </button>
            <button
              onClick={() => setPage((p) => Math.min(totalPages() - 1, p + 1))}
              disabled={page() >= totalPages() - 1}
              class="btn btn-secondary py-1.5 px-3"
            >
              Next
              <ChevronRight size={14} />
            </button>
          </div>
        </div>
      </Show>
    </div>
  );
};

export default Devices;
