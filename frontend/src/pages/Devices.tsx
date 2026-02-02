import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For } from 'solid-js';
import { A } from '@solidjs/router';
import { RefreshCw, ChevronLeft, ChevronRight, Router as RouterIcon } from 'lucide-solid';
import { api } from '../lib/api';

const TableSkeleton: Component = () => (
  <tr>
    <td class="px-4 py-3"><div class="skeleton h-4 w-32" /></td>
    <td class="px-4 py-3"><div class="skeleton h-4 w-40" /></td>
    <td class="px-4 py-3"><div class="skeleton h-4 w-24" /></td>
    <td class="px-4 py-3"><div class="skeleton h-5 w-14" /></td>
    <td class="px-4 py-3"><div class="skeleton h-4 w-28" /></td>
  </tr>
);

const Devices: Component = () => {
  const [page, setPage] = createSignal(0);
  const limit = 20;

  const [deviceList, { refetch }] = createResource(
    () => page(),
    (p) => api.getDevices(limit, p * limit)
  );

  const totalPages = () => Math.ceil((deviceList()?.total ?? 0) / limit);

  const formatDate = (dateStr: string | null) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString('id-ID');
  };

  return (
    <div class="space-y-5">
      <div class="flex items-center justify-between">
        <h1 class="text-xl font-semibold text-primary">Devices</h1>
        <button
          onClick={() => refetch()}
          class="btn btn-secondary"
        >
          <RefreshCw size={14} />
          <span class="hidden sm:inline">Refresh</span>
        </button>
      </div>

      <div class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full min-w-[600px]">
            <thead>
              <tr class="border-b border-subtle">
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide">Serial Number</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide hidden md:table-cell">Model</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide hidden sm:table-cell">IP Address</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide">Status</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide hidden lg:table-cell">Last Seen</th>
              </tr>
            </thead>
          <tbody>
            <Show
              when={!deviceList.loading}
              fallback={<><TableSkeleton /><TableSkeleton /><TableSkeleton /><TableSkeleton /></>}
            >
              <Show
                when={deviceList()?.devices?.length}
                fallback={
                  <tr>
                    <td colspan="5" class="px-4 py-12 text-center">
                      <RouterIcon size={32} class="mx-auto text-muted mb-2 opacity-50" />
                      <p class="text-muted text-sm">No devices found</p>
                      <p class="text-muted opacity-50 text-xs mt-1">Devices will appear after connecting to the ACS</p>
                    </td>
                  </tr>
                }
              >
                <For each={deviceList()?.devices}>
                  {(device) => (
                    <tr class="border-t border-subtle hover:bg-elevated transition-fast">
                      <td class="px-4 py-3">
                        <A href={`/device/${device.serial_number}`} class="text-teal-500 hover:text-teal-400 font-mono text-sm transition-fast">
                          {device.serial_number}
                        </A>
                      </td>
                      <td class="px-4 py-3 text-secondary text-sm hidden md:table-cell">
                        {device.manufacturer} {device.model_name || device.product_class}
                      </td>
                      <td class="px-4 py-3 text-muted font-mono text-sm hidden sm:table-cell">
                        {device.ip_address || '-'}
                      </td>
                      <td class="px-4 py-3">
                        <span class={`badge ${device.online ? 'badge-success' : 'badge-error'}`}>
                          {device.online ? 'Online' : 'Offline'}
                        </span>
                      </td>
                      <td class="px-4 py-3 text-muted text-xs hidden lg:table-cell">
                        {formatDate(device.last_inform)}
                      </td>
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
