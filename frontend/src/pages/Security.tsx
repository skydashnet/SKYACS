import type { Component } from 'solid-js';
import { createResource, createSignal, For, Show } from 'solid-js';
import { Ban, RefreshCw, Trash2 } from 'lucide-solid';
import PageHeader from '../components/PageHeader';
import { useFeedback } from '../components/Feedback';
import { EmptyState, ResourceError } from '../components/ResourceState';
import { api } from '../lib/api';

const Security: Component = () => {
  const { confirm, notify } = useFeedback();
  const [overview, { refetch: refetchOverview }] = createResource(() => api.getSecurityOverview());
  const [audit, { refetch: refetchAudit }] = createResource(() => api.getAuditLogs());
  const [blocked, { refetch: refetchBlocked }] = createResource(() => api.getBlockedDevices());
  const [serial, setSerial] = createSignal('');
  const [reason, setReason] = createSignal('');
  const [saving, setSaving] = createSignal(false);
  const [pendingUnblock, setPendingUnblock] = createSignal<string | null>(null);
  const [message, setMessage] = createSignal<{ type: 'success' | 'error'; text: string } | null>(null);

  const refreshAll = () => { refetchOverview(); refetchAudit(); refetchBlocked(); };
  const blockDevice = async (event: SubmitEvent) => {
    event.preventDefault();
    setSaving(true);
    setMessage(null);
    try {
      await api.blockDevice(serial().trim(), reason().trim());
      setSerial(''); setReason('');
      setMessage({ type: 'success', text: 'Device admission blocked.' });
      refetchBlocked(); refetchAudit();
    } catch (error) {
      notify({ tone: 'error', title: 'CPE was not added to the blocklist', message: 'The serial and reason are preserved. Review the input and retry.', detail: (error as Error).message, persistent: true });
    } finally { setSaving(false); }
  };
  const unblock = async (serialNumber: string) => {
    if (!await confirm({ title: `Allow ${serialNumber} to reconnect?`, description: 'The CPE will be admitted again when its next CWMP session reaches SKYACS. The unblock event remains in the audit trail.', confirmLabel: 'Unblock CPE' })) return;
    if (pendingUnblock()) return;
    setPendingUnblock(serialNumber);
    try {
      await api.unblockDevice(serialNumber);
      notify({ tone: 'success', title: 'CPE removed from admission blocklist', message: serialNumber });
      refetchBlocked(); refetchAudit();
    } catch (error) { notify({ tone: 'error', title: 'Could not unblock CPE', message: 'The admission rule remains unchanged.', detail: (error as Error).message, persistent: true }); }
    finally { setPendingUnblock(null); }
  };
  const formatDate = (value: string) => new Date(value).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });

  return (
    <div class="space-y-5">
      <PageHeader title="Security center" description="Authentication posture, device admission, and operator audit trail.">
        <button class="btn btn-secondary" onClick={refreshAll} disabled={overview.loading || audit.loading || blocked.loading}><RefreshCw size={14} />Refresh security data</button>
      </PageHeader>

      <Show when={message()}><div role={message()!.type === 'error' ? 'alert' : 'status'} class={`px-3 py-2.5 border rounded-[3px] text-xs ${message()!.type === 'success' ? 'border-emerald-500/25 bg-emerald-500/8 text-emerald-400' : 'border-red-500/25 bg-red-500/8 text-red-400'}`}>{message()!.text}</div></Show>

      <Show when={overview.error || audit.error || blocked.error}>
        <div class="card"><ResourceError title="Security data is incomplete" description="One or more security registers could not be loaded. Retry before making an admission or operator decision." onRetry={refreshAll} /></div>
      </Show>

      <dl class="ops-register security-register" aria-label="Security posture register">
        <div class="ops-register-cell"><dt>Operators</dt><dd>{overview()?.total_users ?? '—'}</dd><small>{overview()?.full_access_admins ?? 0} full-access admins</small></div>
        <div class="ops-register-cell"><dt>Failed events</dt><dd>{overview()?.recorded_failures ?? '—'}</dd><small>Recorded in audit trail</small></div>
        <div class={`ops-register-cell is-text ${overview() && !overview()?.jwt_configured ? 'is-warning' : ''}`}><dt>JWT signing</dt><dd>{overview() ? overview()?.jwt_configured ? 'Configured' : 'Review required' : '—'}</dd><small>{overview()?.jwt_configured ? 'Signing secret is active' : 'Verify the signing-secret configuration'}</small></div>
        <div class={`ops-register-cell is-text ${overview() && (!overview()?.cors_restricted || !overview()?.login_rate_limit_enabled) ? 'is-warning' : ''}`}><dt>API controls</dt><dd>{overview() ? overview()?.cors_restricted && overview()?.login_rate_limit_enabled ? 'Restricted' : 'Review required' : '—'}</dd><small>{overview()?.cors_restricted && overview()?.login_rate_limit_enabled ? 'CORS and login throttling active' : 'Verify CORS and login throttling'}</small></div>
      </dl>

      <div class="grid xl:grid-cols-[.8fr_1.2fr] gap-4">
        <section class="data-panel overflow-hidden">
          <div class="p-4 border-b border-subtle"><h3 class="text-sm font-semibold">Device admission blocklist</h3><p class="text-[11px] text-muted mt-1">Blocked serials are rejected before registration or parameter processing.</p></div>
          <form onSubmit={blockDevice} class="p-4 grid sm:grid-cols-2 gap-3 border-b border-subtle bg-elevated/20">
            <div><label for="block-serial" class="block text-[11px] text-muted mb-1.5">CPE serial number</label><input id="block-serial" class="input" value={serial()} onInput={(event) => setSerial(event.currentTarget.value)} required maxlength={128} placeholder="48575443…" /></div>
            <div><label for="block-reason" class="block text-[11px] text-muted mb-1.5">Admission reason</label><input id="block-reason" class="input" value={reason()} onInput={(event) => setReason(event.currentTarget.value)} maxlength={512} placeholder="Unrecognized deployment inventory" /></div>
            <button class="btn btn-danger sm:col-span-2" disabled={saving()}><Ban size={14} />{saving() ? 'Blocking…' : 'Block device'}</button>
          </form>
          <Show when={blocked.loading}><div class="p-4 space-y-3"><div class="skeleton h-8 w-full" /><div class="skeleton h-8 w-4/5" /></div></Show>
          <Show when={!blocked.loading && !blocked.error && (blocked()?.length ?? 0) > 0} fallback={!blocked.loading && !blocked.error ? <EmptyState compact title="Admission blocklist is empty" description="No CPE serial numbers are currently denied by an operator rule." /> : undefined}>
            <div class="max-h-80 overflow-auto"><For each={blocked()}>{(device) => <div class="flex items-center gap-3 px-4 py-3 border-b border-subtle last:border-0"><Ban size={14} class="text-red-400 shrink-0" /><div class="min-w-0 flex-1"><strong class="block font-mono text-xs truncate">{device.serial_number}</strong><span class="block text-[10px] text-muted truncate">{device.reason || 'No reason recorded'} · {device.created_by}</span></div><button class="icon-button" aria-label={`Unblock CPE ${device.serial_number}`} onClick={() => unblock(device.serial_number)} disabled={pendingUnblock() !== null}><Trash2 size={14} /></button></div>}</For></div>
          </Show>
        </section>

        <section class="data-panel overflow-hidden">
          <div class="p-4 border-b border-subtle flex items-center justify-between"><div><h3 class="text-sm font-semibold">Operator audit trail</h3><p class="text-[11px] text-muted mt-1">Append-only record of login and mutation activity.</p></div><span class="badge badge-success">{audit()?.total ?? 0} events</span></div>
          <div class="overflow-x-auto max-h-[480px] overflow-y-auto">
            <table class="data-table min-w-[720px]"><thead class="sticky top-0"><tr><th class="text-left px-4 py-2.5">Time</th><th class="text-left px-4 py-2.5">Actor</th><th class="text-left px-4 py-2.5">Action</th><th class="text-left px-4 py-2.5">Resource</th><th class="text-left px-4 py-2.5">Source</th><th class="text-right px-4 py-2.5">HTTP status</th></tr></thead>
              <tbody><For each={audit()?.entries}>{(entry) => <tr class="border-t border-subtle"><td class="px-4 py-2.5 whitespace-nowrap text-[10px] text-muted">{formatDate(entry.created_at)}</td><td class="px-4 py-2.5 text-xs">{entry.username || 'unknown'}</td><td class="px-4 py-2.5"><span class="font-mono text-[10px] text-sky-500">{entry.action}</span></td><td class="px-4 py-2.5 font-mono text-[10px] text-secondary max-w-[210px] truncate" title={entry.resource}>{entry.resource}</td><td class="px-4 py-2.5 font-mono text-[10px] text-muted">{entry.ip_address || '—'}</td><td class="px-4 py-2.5 text-right"><span class={`badge ${entry.status >= 400 ? 'badge-error' : 'badge-success'}`}>{entry.status}</span></td></tr>}</For></tbody>
            </table>
            <Show when={audit.loading}><div class="p-4 space-y-3"><div class="skeleton h-8 w-full" /><div class="skeleton h-8 w-4/5" /></div></Show>
            <Show when={!audit.loading && !audit.error && (audit()?.entries.length ?? 0) === 0}><EmptyState compact title="No operator security events are recorded" description="Authentication and mutation events will appear after operators begin using this deployment." /></Show>
          </div>
        </section>
      </div>
    </div>
  );
};

export default Security;
