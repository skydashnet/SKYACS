import type { Component } from 'solid-js';
import { createMemo, createResource, For, Show } from 'solid-js';
import { A } from '@solidjs/router';
import { RefreshCw, Router } from 'lucide-solid';
import PageHeader from '../components/PageHeader';
import { EmptyState, ResourceError } from '../components/ResourceState';
import { api } from '../lib/api';

interface DistributionListProps {
  data: Record<string, number>;
  limit?: number;
  semantic?: boolean;
}

const semanticTone = (label: string) => {
  const normalized = label.toLowerCase();
  if (normalized.includes('critical') || normalized.includes('poor')) return 'is-error';
  if (normalized.includes('warning') || normalized.includes('fair')) return 'is-warning';
  if (normalized.includes('good') || normalized.includes('normal') || normalized.includes('30 days')) return 'is-success';
  if (normalized.includes('unknown')) return 'is-muted';
  return '';
};

const DistributionList: Component<DistributionListProps> = (props) => {
  const entries = createMemo(() => Object.entries(props.data)
    .filter(([, value]) => value > 0)
    .sort((a, b) => b[1] - a[1])
    .slice(0, props.limit ?? Number.POSITIVE_INFINITY));
  const maximum = createMemo(() => Math.max(...entries().map(([, value]) => value), 1));

  return (
    <Show when={entries().length > 0} fallback={<p class="py-5 text-center text-xs text-muted">No telemetry available.</p>}>
      <ul class="distribution-list">
        <For each={entries()}>{([label, value]) => (
          <li class="distribution-row">
            <span class="distribution-label" title={label}>{label}</span>
            <progress
              class={`distribution-progress ${props.semantic ? semanticTone(label) : ''}`}
              value={value}
              max={maximum()}
              aria-label={`${label}: ${value}`}
            />
            <span class="distribution-value">{value.toLocaleString()}</span>
          </li>
        )}</For>
      </ul>
    </Show>
  );
};

const Dashboard: Component = () => {
  const [dashboard, { refetch }] = createResource(async () => {
    const [stats, analytics] = await Promise.all([api.getDeviceStats(), api.getDeviceAnalytics()]);
    return { stats, analytics, fetchedAt: new Date() };
  });
  const stats = createMemo(() => dashboard()?.stats);
  const analytics = createMemo(() => dashboard()?.analytics);
  const onlinePercent = createMemo(() => {
    const total = stats()?.total ?? 0;
    return total > 0 ? Math.round(((stats()?.online ?? 0) / total) * 1000) / 10 : 0;
  });

  return (
    <div class="space-y-4">
      <PageHeader
        title="Fleet status"
        description="Current fleet reachability, contact recency, health thresholds, and hardware mix."
        status={<span class="inline-flex items-center gap-2 text-[11px] text-muted"><span class="w-1.5 h-1.5 bg-emerald-500" aria-hidden="true" />{dashboard.loading ? 'Refreshing fleet data' : dashboard()?.fetchedAt ? `Updated ${dashboard()!.fetchedAt.toLocaleTimeString()}` : 'Update unavailable'}</span>}
      >
        <button type="button" class="btn btn-secondary" onClick={() => refetch()} disabled={dashboard.loading}><RefreshCw size={14} />Refresh fleet data</button>
      </PageHeader>

      <Show when={!dashboard.loading && !dashboard.error && (stats()?.total ?? 0) === 0}>
        <div class="data-panel"><EmptyState
          icon={<Router size={22} />}
          title="No CPEs have reported to SKYACS"
          description="The fleet register will populate after a device sends its first CWMP Inform. Verify the published CWMP endpoint and the CPE ACS URL."
          action={<A class="btn btn-secondary" href="/settings">Review CWMP connection settings</A>}
        /></div>
      </Show>

      <Show when={!dashboard.loading && !dashboard.error && (analytics()?.total ?? 0) > (analytics()?.sampled ?? 0)}>
        <div class="px-4 py-3 border border-amber-500/30 text-amber-400 text-xs">
          Analytics use the most recent {analytics()?.sampled.toLocaleString()} of {analytics()?.total.toLocaleString()} devices; fleet totals remain exact.
        </div>
      </Show>

      <Show when={dashboard.loading}>
        <div class="ops-register fleet-register" aria-label="Loading fleet status">
          <For each={[1, 2, 3, 4]}>{() => <div class="ops-register-cell"><div class="skeleton h-3 w-24" /><div class="skeleton h-7 w-16 mt-3" /><div class="skeleton h-2 w-32 mt-3" /></div>}</For>
        </div>
        <div class="data-panel"><div class="data-panel-body space-y-3"><div class="skeleton h-3 w-40" /><div class="skeleton h-5 w-full" /><div class="skeleton h-5 w-4/5" /></div></div>
      </Show>

      <Show when={dashboard.error}>
        <div class="data-panel"><ResourceError title="Fleet telemetry is unavailable" description="SKYACS could not read the current device statistics. Confirm the API and database are reachable, then retry." onRetry={() => refetch()} /></div>
      </Show>

      <Show when={!dashboard.loading && !dashboard.error && (stats()?.total ?? 0) > 0}>
        <dl class="ops-register fleet-register" aria-label="Fleet status register">
          <div class="ops-register-cell">
            <dt>Managed devices</dt>
            <dd>{(stats()?.total ?? 0).toLocaleString()}</dd>
            <small>Registered CPE inventory</small>
          </div>
          <div class="ops-register-cell is-online">
            <dt>Online</dt>
            <dd>{(stats()?.online ?? 0).toLocaleString()}</dd>
            <small>Reporting within threshold</small>
          </div>
          <div class="ops-register-cell is-offline">
            <dt>Offline</dt>
            <dd>{(stats()?.offline ?? 0).toLocaleString()}</dd>
            <small>Requires operator review</small>
          </div>
          <div class="ops-register-cell">
            <dt>Fleet availability</dt>
            <dd>{onlinePercent()}%</dd>
            <small>Online share of inventory</small>
          </div>
        </dl>

        <section class="data-panel">
          <div class="data-panel-header">
            <div><h2>Contact recency</h2><p>Time elapsed since the latest Inform received from each CPE.</p></div>
          </div>
          <div class="data-panel-body">
            <DistributionList data={analytics()?.lastInform ?? {}} semantic />
          </div>
        </section>

        <section class="data-panel">
          <div class="data-panel-header">
            <div><h2>Device health distributions</h2><p>Threshold buckets derived from the latest sampled parameter values.</p></div>
          </div>
          <div class="dashboard-health-grid">
            <div class="data-panel-section">
              <h3>Uptime</h3>
              <DistributionList data={analytics()?.uptime ?? {}} semantic />
            </div>
            <div class="data-panel-section">
              <h3>Temperature</h3>
              <DistributionList data={analytics()?.temperature ?? {}} semantic />
            </div>
            <div class="data-panel-section">
              <h3>Optical RX power</h3>
              <DistributionList data={analytics()?.rxPower ?? {}} semantic />
            </div>
          </div>
        </section>

        <section class="data-panel">
          <div class="data-panel-header">
            <div><h2>Inventory composition</h2><p>Access technology, manufacturer, and product-class concentration.</p></div>
          </div>
          <div class="inventory-grid">
            <div class="data-panel-section">
              <h3>Access type</h3>
              <DistributionList data={analytics()?.accessType ?? {}} limit={6} />
            </div>
            <div class="data-panel-section">
              <h3>Manufacturer</h3>
              <DistributionList data={analytics()?.manufacturers ?? {}} limit={6} />
            </div>
            <div class="data-panel-section">
              <h3>Product class</h3>
              <DistributionList data={analytics()?.productClasses ?? {}} limit={6} />
            </div>
          </div>
        </section>
      </Show>
    </div>
  );
};

export default Dashboard;
