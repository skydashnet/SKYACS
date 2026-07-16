import type { Component } from 'solid-js';
import { createResource, createSignal, For, Show } from 'solid-js';
import { Ban, CheckCircle2, FileClock, KeyRound, LockKeyhole, RefreshCw, ShieldCheck, Trash2, Users } from 'lucide-solid';
import { api } from '../lib/api';

const Security: Component = () => {
  const [overview, { refetch: refetchOverview }] = createResource(() => api.getSecurityOverview());
  const [audit, { refetch: refetchAudit }] = createResource(() => api.getAuditLogs());
  const [blocked, { refetch: refetchBlocked }] = createResource(() => api.getBlockedDevices());
  const [serial, setSerial] = createSignal('');
  const [reason, setReason] = createSignal('');
  const [saving, setSaving] = createSignal(false);
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
      setMessage({ type: 'error', text: (error as Error).message });
    } finally { setSaving(false); }
  };
  const unblock = async (serialNumber: string) => {
    if (!confirm(`Allow ${serialNumber} to connect again?`)) return;
    try { await api.unblockDevice(serialNumber); refetchBlocked(); refetchAudit(); }
    catch (error) { setMessage({ type: 'error', text: (error as Error).message }); }
  };
  const formatDate = (value: string) => new Date(value).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });

  return (
    <div class="space-y-5">
      <div class="flex items-start justify-between gap-4">
        <div><p class="text-[10px] uppercase tracking-[.12em] text-sky-500 font-semibold">Access governance</p><h2 class="text-xl font-semibold tracking-[-.02em] mt-1">Security center</h2><p class="text-xs text-muted mt-1">Authentication posture, device admission, and operator audit trail.</p></div>
        <button class="btn btn-secondary" onClick={refreshAll}><RefreshCw size={14} />Refresh</button>
      </div>

      <Show when={message()}><div class={`px-3 py-2.5 border rounded-[3px] text-xs ${message()!.type === 'success' ? 'border-emerald-500/25 bg-emerald-500/8 text-emerald-400' : 'border-red-500/25 bg-red-500/8 text-red-400'}`}>{message()!.text}</div></Show>

      <div class="grid grid-cols-2 xl:grid-cols-4 gap-3">
        <div class="card p-4"><div class="flex justify-between text-muted"><span class="text-[10px] uppercase tracking-wider">Operators</span><Users size={15} /></div><strong class="block mt-4 text-2xl font-mono">{overview()?.total_users ?? '—'}</strong><span class="text-[10px] text-muted">{overview()?.full_access_admins ?? 0} full-access admins</span></div>
        <div class="card p-4"><div class="flex justify-between text-muted"><span class="text-[10px] uppercase tracking-wider">Failed events</span><FileClock size={15} /></div><strong class="block mt-4 text-2xl font-mono">{overview()?.recorded_failures ?? '—'}</strong><span class="text-[10px] text-muted">Recorded in audit trail</span></div>
        <div class="card p-4"><div class="flex justify-between text-muted"><span class="text-[10px] uppercase tracking-wider">JWT signing</span><KeyRound size={15} /></div><strong class="block mt-4 text-sm">Configured</strong><span class="badge badge-success mt-2"><CheckCircle2 size={11} />HS256 enforced</span></div>
        <div class="card p-4"><div class="flex justify-between text-muted"><span class="text-[10px] uppercase tracking-wider">API posture</span><ShieldCheck size={15} /></div><strong class="block mt-4 text-sm">Restricted</strong><span class="badge badge-success mt-2"><LockKeyhole size={11} />RBAC + CORS</span></div>
      </div>

      <div class="grid xl:grid-cols-[.8fr_1.2fr] gap-4">
        <section class="card overflow-hidden">
          <div class="p-4 border-b border-subtle"><h3 class="text-sm font-semibold">Device admission blocklist</h3><p class="text-[11px] text-muted mt-1">Blocked serials are rejected before registration or parameter processing.</p></div>
          <form onSubmit={blockDevice} class="p-4 grid sm:grid-cols-2 gap-3 border-b border-subtle bg-elevated/20">
            <div><label class="block text-[10px] uppercase tracking-wider text-muted mb-1.5">Serial number</label><input class="input" value={serial()} onInput={(event) => setSerial(event.currentTarget.value)} required maxlength={128} placeholder="48575443…" /></div>
            <div><label class="block text-[10px] uppercase tracking-wider text-muted mb-1.5">Reason</label><input class="input" value={reason()} onInput={(event) => setReason(event.currentTarget.value)} maxlength={512} placeholder="Unauthorized CPE" /></div>
            <button class="btn btn-danger sm:col-span-2" disabled={saving()}><Ban size={14} />{saving() ? 'Blocking…' : 'Block device'}</button>
          </form>
          <Show when={(blocked()?.length ?? 0) > 0} fallback={<div class="p-8 text-center text-xs text-muted">No blocked devices.</div>}>
            <div class="max-h-80 overflow-auto"><For each={blocked()}>{(device) => <div class="flex items-center gap-3 px-4 py-3 border-b border-subtle last:border-0"><Ban size={14} class="text-red-400 shrink-0" /><div class="min-w-0 flex-1"><strong class="block font-mono text-xs truncate">{device.serial_number}</strong><span class="block text-[10px] text-muted truncate">{device.reason || 'No reason'} · {device.created_by}</span></div><button class="icon-button" title="Unblock" onClick={() => unblock(device.serial_number)}><Trash2 size={14} /></button></div>}</For></div>
          </Show>
        </section>

        <section class="card overflow-hidden">
          <div class="p-4 border-b border-subtle flex items-center justify-between"><div><h3 class="text-sm font-semibold">Operator audit trail</h3><p class="text-[11px] text-muted mt-1">Append-only record of login and mutation activity.</p></div><span class="badge badge-success">{audit()?.total ?? 0} events</span></div>
          <div class="overflow-x-auto max-h-[480px] overflow-y-auto">
            <table class="min-w-[720px]"><thead class="sticky top-0"><tr><th class="text-left px-4 py-2.5">Time</th><th class="text-left px-4 py-2.5">Actor</th><th class="text-left px-4 py-2.5">Action</th><th class="text-left px-4 py-2.5">Resource</th><th class="text-left px-4 py-2.5">Source</th><th class="text-right px-4 py-2.5">Status</th></tr></thead>
              <tbody><For each={audit()?.entries}>{(entry) => <tr class="border-t border-subtle"><td class="px-4 py-2.5 whitespace-nowrap text-[10px] text-muted">{formatDate(entry.created_at)}</td><td class="px-4 py-2.5 text-xs">{entry.username || 'unknown'}</td><td class="px-4 py-2.5"><span class="font-mono text-[10px] text-sky-500">{entry.action}</span></td><td class="px-4 py-2.5 font-mono text-[10px] text-secondary max-w-[210px] truncate" title={entry.resource}>{entry.resource}</td><td class="px-4 py-2.5 font-mono text-[10px] text-muted">{entry.ip_address || '—'}</td><td class="px-4 py-2.5 text-right"><span class={`badge ${entry.status >= 400 ? 'badge-error' : 'badge-success'}`}>{entry.status}</span></td></tr>}</For></tbody>
            </table>
            <Show when={!audit.loading && (audit()?.entries.length ?? 0) === 0}><div class="p-10 text-center text-xs text-muted">No security events recorded yet.</div></Show>
          </div>
        </section>
      </div>
    </div>
  );
};

export default Security;
