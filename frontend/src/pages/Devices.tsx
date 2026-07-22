import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For, createMemo, createEffect, onMount } from 'solid-js';
import { A } from '@solidjs/router';
import { RefreshCw, ChevronLeft, ChevronRight, Router as RouterIcon, Settings2, X, Check, GripVertical, Search, Send, ArrowUp, ArrowDown, ArrowUpDown } from 'lucide-solid';
import { api, type Device } from '../lib/api';
import { useAuth } from '../lib/auth';
import PageHeader from '../components/PageHeader';
import { useFeedback } from '../components/Feedback';
import { EmptyState, ResourceError } from '../components/ResourceState';

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

const STORAGE_KEY = 'skyacs_device_columns';

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
  const [searchQuery, setSearchQuery] = createSignal('');
  const [columns, setColumns] = createSignal<ColumnConfig[]>([]);
  const [summoningAll, setSummoningAll] = createSignal(false);
  const [sortBy, setSortBy] = createSignal<{ column: string; direction: 'asc' | 'desc' } | null>(null);
  const { isFullAccess } = useAuth();
  const { notify } = useFeedback();
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
    () => page(),
    (p) => api.getDevices(limit, p * limit)
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

  const moveColumn = (id: string, offset: -1 | 1) => {
    setColumns((current) => {
      const sorted = [...current].sort((a, b) => a.order - b.order);
      const index = sorted.findIndex((column) => column.id === id);
      const target = index + offset;
      if (index < 0 || target < 0 || target >= sorted.length) return current;
      [sorted[index], sorted[target]] = [sorted[target]!, sorted[index]!];
      return sorted.map((column, order) => ({ ...column, order }));
    });
  };

  const getRxPower = (device: Device) => {
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

  const getPppUsername = (device: Device) => {
    const param = device.parameters?.find((p: any) => 
      p.name.includes('WANPPPConnection') && p.name.includes('Username')
    );
    return param?.value || '-';
  };

  const getCellValue = (device: Device, columnId: string) => {
    switch (columnId) {
      case 'serial_number':
        return (
          <A class="data-link font-mono text-sm" href={`/device/${encodeURIComponent(device.serial_number)}`} aria-label={`Open CPE ${device.serial_number}`}>
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
          <span class={`inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium ${
            device.online 
              ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' 
              : 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
          }`}>
            <span class={`w-1.5 h-1.5 rounded-full ${device.online ? 'bg-emerald-400' : 'bg-rose-400'}`} aria-hidden="true" />
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
    const query = searchQuery().toLowerCase().trim();
    if (!query) return devices;
    
    return devices.filter(d => {
      return (
        d.serial_number?.toLowerCase().includes(query) ||
        d.manufacturer?.toLowerCase().includes(query) ||
        d.model_name?.toLowerCase().includes(query) ||
        d.product_class?.toLowerCase().includes(query) ||
        d.ip_address?.toLowerCase().includes(query)
      );
    });
  });

  const sortedDevices = createMemo(() => {
    const devices = [...filteredDevices()];
    const sort = sortBy();
    if (!sort) return devices;
    
    return devices.sort((a, b) => {
      let aVal: any, bVal: any;
      
      switch (sort.column) {
        case 'serial_number':
          aVal = a.serial_number || '';
          bVal = b.serial_number || '';
          break;
        case 'manufacturer':
          aVal = a.manufacturer || '';
          bVal = b.manufacturer || '';
          break;
        case 'model':
          aVal = a.model_name || a.product_class || '';
          bVal = b.model_name || b.product_class || '';
          break;
        case 'ip_address':
          aVal = a.ip_address || '';
          bVal = b.ip_address || '';
          break;
        case 'status':
          aVal = a.online ? 1 : 0;
          bVal = b.online ? 1 : 0;
          break;
        case 'last_inform':
          aVal = a.last_inform ? new Date(a.last_inform).getTime() : 0;
          bVal = b.last_inform ? new Date(b.last_inform).getTime() : 0;
          break;
        default:
          return 0;
      }
      
      if (typeof aVal === 'string') {
        const comparison = aVal.localeCompare(bVal);
        return sort.direction === 'asc' ? comparison : -comparison;
      }
      
      return sort.direction === 'asc' ? aVal - bVal : bVal - aVal;
    });
  });

  const handleSort = (columnId: string) => {
    const current = sortBy();
    if (current?.column === columnId) {
      if (current.direction === 'asc') {
        setSortBy({ column: columnId, direction: 'desc' });
      } else {
        setSortBy(null);
      }
    } else {
      setSortBy({ column: columnId, direction: 'asc' });
    }
  };

  return (
    <div class="space-y-5">
      <PageHeader title="CPE inventory" description="Search, inspect, and operate registered TR-069 endpoints.">
        <div class="flex gap-2">
          <div class="flex flex-col gap-1">
            <label for="device-search" class="text-[10px] text-muted">Find CPE</label>
            <div class="relative">
            <Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-muted pointer-events-none" />
            <input
              id="device-search"
              type="text"
              value={searchQuery()}
              onInput={(e) => setSearchQuery(e.currentTarget.value)}
              placeholder="Serial, model, vendor, or IP"
              class="input device-search-input w-48 text-sm"
            />
            <Show when={searchQuery()}>
              <button
                onClick={() => setSearchQuery('')}
                class="input-clear"
                aria-label="Clear CPE search"
              >
                <X size={12} />
              </button>
            </Show>
            </div>
          </div>
          <button onClick={() => setShowColumnSettings(!showColumnSettings())}
            class={`btn btn-secondary ${showColumnSettings() ? 'bg-sky-500/20 text-sky-400' : ''}`}
            aria-expanded={showColumnSettings()}
            aria-controls="device-column-settings"
          >
            <Settings2 size={14} />
            <span class="hidden sm:inline">Columns</span>
          </button>
          <Show when={isFullAccess()}><button
            onClick={async () => {
              const list = deviceList()?.devices;
              if (!list || summoningAll()) return;
              setSummoningAll(true);
              try {
                const results = await Promise.allSettled(list.map((device) => api.connectionRequest(device.serial_number)));
                const successCount = results.filter((result) => result.status === 'fulfilled').length;
                const failureCount = list.length - successCount;
                notify({
                  tone: failureCount > 0 ? 'error' : 'success',
                  title: failureCount > 0 ? 'Connection requests partially completed' : 'Connection requests sent',
                  message: `${successCount} of ${list.length} visible CPEs accepted the request.${failureCount > 0 ? ` ${failureCount} require individual review.` : ''}`,
                  persistent: failureCount > 0,
                });
                await refetch();
              } finally {
                setSummoningAll(false);
              }
            }}
            disabled={summoningAll()}
            class="btn btn-secondary"
          >
            <Send size={14} class={summoningAll() ? 'animate-pulse' : ''} />
            <span class="hidden sm:inline">{summoningAll() ? 'Summoning...' : 'Summon All'}</span>
          </button></Show>
          <button onClick={() => refetch()} class="btn btn-secondary" disabled={deviceList.loading}>
            <RefreshCw size={14} />
            <span class="hidden sm:inline">Refresh</span>
          </button>
        </div>
      </PageHeader>

      {/* Column Settings Panel */}
      <Show when={showColumnSettings()}>
        <div id="device-column-settings" class="card p-4">
          <div class="flex items-center justify-between mb-3">
            <h3 class="text-sm font-medium text-secondary">Manage Columns</h3>
            <button onClick={() => setShowColumnSettings(false)} class="icon-button" aria-label="Close column settings">
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
                  class={`flex items-center gap-2 px-3 py-1.5 bg-elevated cursor-grab active:cursor-grabbing transition-all ${draggedCol() === col.id ? 'opacity-50 scale-95' : 'hover:bg-elevated/80'}`}
                >
                  <GripVertical size={12} class="text-muted" />
                  <button
                    onClick={() => toggleColumn(col.id)}
                    class={`icon-button ${col.visible ? 'text-sky-400 border-sky-500' : ''}`}
                    aria-label={`${col.visible ? 'Hide' : 'Show'} ${col.label} column`}
                    aria-pressed={col.visible}
                  >
                    {col.visible && <Check size={10} class="text-white" />}
                  </button>
                  <span class={`text-sm ${col.visible ? 'text-primary' : 'text-muted'}`}>
                    {col.label}
                  </span>
                  <span class="inline-flex ml-auto">
                    <button type="button" class="icon-button" onClick={() => moveColumn(col.id, -1)} aria-label={`Move ${col.label} column left`}><ChevronLeft size={12} /></button>
                    <button type="button" class="icon-button" onClick={() => moveColumn(col.id, 1)} aria-label={`Move ${col.label} column right`}><ChevronRight size={12} /></button>
                  </span>
                </div>
              )}
            </For>
          </div>
        </div>
      </Show>

      <Show when={deviceList.error}>
        <div class="card"><ResourceError title="CPE inventory is unavailable" description="The inventory request failed. Check the API service status and retry without losing the current search or column settings." onRetry={() => refetch()} /></div>
      </Show>

      <Show when={!deviceList.error}><div class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="data-table w-full">
            <thead>
              <tr class="border-b border-subtle">
                <For each={visibleColumns()}>
                  {(col) => {
                    const isSortable = ['serial_number', 'manufacturer', 'model', 'ip_address', 'status', 'last_inform'].includes(col.id);
                    const currentSort = sortBy();
                    const isSorted = currentSort?.column === col.id;
                    
                    return (
                      <th class="px-4 py-2 text-left whitespace-nowrap" aria-sort={isSorted ? (currentSort?.direction === 'asc' ? 'ascending' : 'descending') : undefined}>
                        <div class="flex items-center gap-2">
                          <button
                            onClick={(e) => {
                              e.preventDefault();
                              e.stopPropagation();
                              if (isSortable) handleSort(col.id);
                            }}
                            class={`flex items-center gap-1 text-xs font-medium tracking-wide transition-colors ${
                              isSorted ? 'text-sky-400' : 'text-muted'
                            } ${isSortable ? 'hover:text-primary cursor-pointer' : 'cursor-default'}`}
                            disabled={!isSortable}
                          >
                            {col.label}
                            <Show when={isSortable}>
                              {isSorted ? (
                                currentSort?.direction === 'asc' ? <ArrowUp size={12} /> : <ArrowDown size={12} />
                              ) : (
                                <ArrowUpDown size={10} class="opacity-50" />
                              )}
                            </Show>
                          </button>
                        </div>
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
                  when={sortedDevices().length}
                  fallback={
                    <tr>
                      <td colspan={visibleColumns().length} class="px-4 py-12 text-center">
                        <EmptyState
                          compact
                          icon={<RouterIcon size={22} />}
                          title={searchQuery() ? 'No CPEs match this search' : 'No CPEs have registered'}
                          description={searchQuery() ? 'Change the serial, model, vendor, or IP search to return to the current inventory.' : 'CPEs appear after their first CWMP Inform reaches SKYACS. Verify the published endpoint and device ACS URL.'}
                          action={searchQuery() ? <button type="button" class="btn btn-secondary" onClick={() => setSearchQuery('')}>Clear search</button> : undefined}
                        />
                      </td>
                    </tr>
                  }
                >
                  <For each={sortedDevices()}>
                    {(device, idx) => (
                      <tr class={`border-t border-subtle hover:bg-elevated transition-fast ${idx() % 2 === 1 ? 'bg-base/50' : ''}`}>
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
      </div></Show>

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
