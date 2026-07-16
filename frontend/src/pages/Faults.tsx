import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For } from 'solid-js';
import { A } from '@solidjs/router';
import { AlertTriangle, CheckCircle, Trash2, RefreshCw } from 'lucide-solid';
import { api } from '../lib/api';
import { useAuth } from '../lib/auth';

const Faults: Component = () => {
  const { isFullAccess } = useAuth();
  const [filter, setFilter] = createSignal<'all' | 'active' | 'resolved'>('active');
  const [faults, { refetch }] = createResource(
    () => filter(),
    (selected) => api.getFaults(selected)
  );

  const [stats, { refetch: refetchStats }] = createResource(() => api.getFaultStats());

  const handleResolve = async (id: number) => {
    try { await api.resolveFault(id); refetch(); refetchStats(); } catch (error) { alert((error as Error).message); }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Yakin hapus fault ini?')) return;
    try { await api.deleteFault(id); refetch(); refetchStats(); } catch (error) { alert((error as Error).message); }
  };

  const formatDate = (date: string) => {
    return new Date(date).toLocaleString('id-ID');
  };

  const getFaultColor = (code: string) => {
    const num = parseInt(code);
    if (num >= 9000) return 'text-rose-400';
    if (num >= 8000) return 'text-orange-400';
    return 'text-amber-400';
  };

  return (
    <div class="space-y-6">
      <div class="flex items-center justify-between">
        <div><p class="text-[10px] uppercase tracking-[.12em] text-sky-500 font-semibold">CWMP diagnostics</p><h1 class="text-xl font-semibold text-primary mt-1">Fault center</h1><p class="text-xs text-muted mt-1">Investigate and resolve device-side protocol failures.</p></div>
        <button onClick={() => refetch()} class="btn btn-secondary">
          <RefreshCw size={14} />
          Refresh
        </button>
      </div>

      {/* Stats */}
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div class="card p-5">
          <div class="flex items-center gap-2 text-muted text-sm mb-2">
            <AlertTriangle size={14} />
            <span>Active Faults</span>
          </div>
          <p class="text-2xl font-semibold text-rose-400">
            {stats()?.active ?? 0}
          </p>
        </div>
        <div class="card p-5">
          <div class="flex items-center gap-2 text-muted text-sm mb-2">
            <CheckCircle size={14} />
            <span>Resolved</span>
          </div>
          <p class="text-2xl font-semibold text-emerald-400">
            {stats()?.resolved ?? 0}
          </p>
        </div>
        <div class="card p-5">
          <div class="flex items-center gap-2 text-muted text-sm mb-2">
            <AlertTriangle size={14} />
            <span>Total</span>
          </div>
          <p class="text-2xl font-semibold text-primary">
            {stats()?.total ?? 0}
          </p>
        </div>
      </div>

      {/* Filter */}
      <div class="flex gap-2">
        <button 
          onClick={() => setFilter('active')}
          class={`px-4 py-2 rounded-md text-sm transition-fast ${
            filter() === 'active' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'bg-elevated text-secondary hover:bg-zinc-700'
          }`}
        >
          Active
        </button>
        <button 
          onClick={() => setFilter('resolved')}
          class={`px-4 py-2 rounded-md text-sm transition-fast ${
            filter() === 'resolved' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'bg-elevated text-secondary hover:bg-zinc-700'
          }`}
        >
          Resolved
        </button>
        <button 
          onClick={() => setFilter('all')}
          class={`px-4 py-2 rounded-md text-sm transition-fast ${
            filter() === 'all' ? 'bg-zinc-600 text-primary' : 'bg-elevated text-secondary hover:bg-zinc-700'
          }`}
        >
          All
        </button>
      </div>

      {/* Faults List */}
      <div class="card p-0 overflow-hidden">
        <Show when={!faults.loading} fallback={
          <div class="p-8 text-center text-muted">Loading...</div>
        }>
          <Show when={(faults()?.faults?.length ?? 0) > 0} fallback={
            <div class="p-8 text-center">
              <AlertTriangle size={32} class="mx-auto text-muted opacity-50 mb-2" />
              <p class="text-muted text-sm">No faults found</p>
              <p class="text-muted text-xs mt-1">
                Faults akan muncul ketika device mengirim error response
              </p>
            </div>
          }>
            <div class="overflow-x-auto">
              <table class="w-full text-sm min-w-[700px]">
                <thead>
                  <tr class="border-b border-subtle bg-surface/50">
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted uppercase">Device</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted uppercase">Code</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted uppercase">Message</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted uppercase hidden lg:table-cell">Parameter</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted uppercase hidden md:table-cell">Time</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted uppercase">Status</th>
                    <th class="text-right px-4 py-3 text-xs font-medium text-muted uppercase">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  <For each={faults()?.faults}>
                    {(fault) => (
                      <tr class="border-t border-subtle/50 hover:bg-elevated/30">
                        <td class="px-4 py-3">
                          <A href={`/device/${fault.serial_number}`} class="text-sky-400 hover:underline font-mono text-xs">
                            {fault.serial_number}
                          </A>
                        </td>
                        <td class="px-4 py-3">
                          <span class={`font-mono font-semibold ${getFaultColor(fault.fault_code)}`}>
                            {fault.fault_code}
                          </span>
                        </td>
                        <td class="px-4 py-3 text-secondary max-w-xs truncate" title={fault.fault_string}>
                          {fault.fault_string.length > 40 ? fault.fault_string.slice(0, 40) + '...' : fault.fault_string}
                        </td>
                        <td class="px-4 py-3 text-muted font-mono text-xs max-w-xs truncate hidden lg:table-cell" title={fault.parameter_name}>
                          {fault.parameter_name || '-'}
                        </td>
                        <td class="px-4 py-3 text-muted text-xs hidden md:table-cell">
                          {formatDate(fault.created_at)}
                        </td>
                        <td class="px-4 py-3">
                          <span class={`badge ${fault.resolved ? 'badge-success' : 'badge-error'}`}>
                            {fault.resolved ? 'Resolved' : 'Active'}
                          </span>
                        </td>
                        <td class="px-4 py-3 text-right">
                          <div class="flex items-center justify-end gap-2">
                            <Show when={!fault.resolved && isFullAccess()}>
                              <button 
                                onClick={() => handleResolve(fault.id)}
                                class="p-1.5 rounded hover:bg-emerald-500/20 text-muted hover:text-emerald-400 transition-fast"
                                title="Mark as resolved"
                              >
                                <CheckCircle size={14} />
                              </button>
                            </Show>
                            <Show when={isFullAccess()}><button
                              onClick={() => handleDelete(fault.id)}
                              class="p-1.5 rounded hover:bg-rose-500/20 text-muted hover:text-rose-400 transition-fast"
                              title="Delete"
                            >
                              <Trash2 size={14} />
                            </button></Show>
                          </div>
                        </td>
                      </tr>
                    )}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>
        </Show>
      </div>
    </div>
  );
};

export default Faults;
