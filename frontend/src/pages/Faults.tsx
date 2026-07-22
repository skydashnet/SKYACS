import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For } from 'solid-js';
import { A } from '@solidjs/router';
import { AlertTriangle, CheckCircle, Trash2, RefreshCw } from 'lucide-solid';
import { api } from '../lib/api';
import { useAuth } from '../lib/auth';
import PageHeader from '../components/PageHeader';
import { useFeedback } from '../components/Feedback';
import { EmptyState, ResourceError } from '../components/ResourceState';

const Faults: Component = () => {
  const { isFullAccess } = useAuth();
  const { confirm, notify } = useFeedback();
  const [filter, setFilter] = createSignal<'all' | 'active' | 'resolved'>('active');
  const [pendingFault, setPendingFault] = createSignal<number | null>(null);
  const [faults, { refetch }] = createResource(
    () => filter(),
    (selected) => api.getFaults(selected)
  );

  const [stats, { refetch: refetchStats }] = createResource(() => api.getFaultStats());

  const handleResolve = async (id: number) => {
    setPendingFault(id);
    try {
      await api.resolveFault(id);
      notify({ tone: 'success', title: 'Fault marked as resolved', message: 'The audit trail and fault register have been updated.' });
      await Promise.all([refetch(), refetchStats()]);
    } catch (error) {
      notify({ tone: 'error', title: 'Could not resolve fault', message: 'The fault remains active. Retry after checking the API service.', detail: (error as Error).message, persistent: true });
    } finally { setPendingFault(null); }
  };

  const handleDelete = async (id: number) => {
    if (!await confirm({ title: 'Delete fault record?', description: 'This permanently removes the selected protocol fault from the operational history. This action cannot be undone.', confirmLabel: 'Delete fault', tone: 'danger' })) return;
    setPendingFault(id);
    try {
      await api.deleteFault(id);
      notify({ tone: 'success', title: 'Fault record deleted' });
      await Promise.all([refetch(), refetchStats()]);
    } catch (error) {
      notify({ tone: 'error', title: 'Could not delete fault', message: 'No local state was changed. Retry after checking the API service.', detail: (error as Error).message, persistent: true });
    } finally { setPendingFault(null); }
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
      <PageHeader title="Fault center" description="Investigate and resolve device-side protocol failures.">
        <button onClick={() => { refetch(); refetchStats(); }} class="btn btn-secondary" disabled={faults.loading || stats.loading}>
          <RefreshCw size={14} />
          Refresh
        </button>
      </PageHeader>

      <Show when={faults.error || stats.error}>
        <div class="card"><ResourceError title="Fault data is unavailable" description="SKYACS could not read the protocol fault register. Retry the request without changing the selected filter." onRetry={() => { refetch(); refetchStats(); }} /></div>
      </Show>

      <dl class="ops-register fault-register" aria-label="Fault status register" aria-busy={stats.loading}>
        <div class="ops-register-cell is-offline"><dt>Active faults</dt><dd>{stats()?.active ?? 0}</dd><small>Awaiting operator action</small></div>
        <div class="ops-register-cell is-online"><dt>Resolved</dt><dd>{stats()?.resolved ?? 0}</dd><small>Closed fault records</small></div>
        <div class="ops-register-cell"><dt>Total recorded</dt><dd>{stats()?.total ?? 0}</dd><small>All CWMP fault events</small></div>
      </dl>

      {/* Filter */}
      <div class="flex gap-2">
        <button 
          onClick={() => setFilter('active')}
          class={`px-4 py-2 rounded-md text-sm transition-fast ${
            filter() === 'active' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'bg-elevated text-secondary hover:bg-zinc-700'
          }`}
          aria-pressed={filter() === 'active'}
        >
          Active
        </button>
        <button 
          onClick={() => setFilter('resolved')}
          class={`px-4 py-2 rounded-md text-sm transition-fast ${
            filter() === 'resolved' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'bg-elevated text-secondary hover:bg-zinc-700'
          }`}
          aria-pressed={filter() === 'resolved'}
        >
          Resolved
        </button>
        <button 
          onClick={() => setFilter('all')}
          class={`px-4 py-2 rounded-md text-sm transition-fast ${
            filter() === 'all' ? 'bg-zinc-600 text-primary' : 'bg-elevated text-secondary hover:bg-zinc-700'
          }`}
          aria-pressed={filter() === 'all'}
        >
          All
        </button>
      </div>

      {/* Faults List */}
      <Show when={!faults.error}><div class="card p-0 overflow-hidden">
        <Show when={!faults.loading} fallback={
          <div class="p-4 space-y-3" aria-label="Loading fault records"><div class="skeleton h-8 w-full" /><div class="skeleton h-8 w-full" /><div class="skeleton h-8 w-4/5" /></div>
        }>
          <Show when={(faults()?.faults?.length ?? 0) > 0} fallback={
            <EmptyState
              icon={<AlertTriangle size={22} />}
              title={filter() === 'active' ? 'No active CWMP faults require review' : `No ${filter()} fault records`}
              description={filter() === 'active' ? 'The current fleet has not reported an unresolved protocol failure.' : 'Change the fault filter to inspect another part of the protocol history.'}
              action={filter() !== 'all' ? <button type="button" class="btn btn-secondary" onClick={() => setFilter('all')}>Show all fault records</button> : undefined}
            />
          }>
            <div class="overflow-x-auto">
              <table class="data-table w-full text-sm min-w-[700px]">
                <thead>
                  <tr class="border-b border-subtle bg-surface/50">
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted">Device</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted">Code</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted">Message</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted hidden lg:table-cell">Parameter</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted hidden md:table-cell">Time</th>
                    <th class="text-left px-4 py-3 text-xs font-medium text-muted">Status</th>
                    <th class="text-right px-4 py-3 text-xs font-medium text-muted">Actions</th>
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
                                class="icon-button hover:text-emerald-400"
                                aria-label={`Mark fault ${fault.fault_code} as resolved`}
                                disabled={pendingFault() === fault.id}
                              >
                                <CheckCircle size={14} />
                              </button>
                            </Show>
                            <Show when={isFullAccess()}><button
                              onClick={() => handleDelete(fault.id)}
                              class="icon-button hover:text-rose-400"
                              aria-label={`Delete fault ${fault.fault_code}`}
                              disabled={pendingFault() === fault.id}
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
      </div></Show>
    </div>
  );
};

export default Faults;
