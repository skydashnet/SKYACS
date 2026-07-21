import type { Component } from 'solid-js';
import { createMemo, createResource, For, Show } from 'solid-js';
import PageHeader from '../components/PageHeader';
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
  const [stats] = createResource(() => api.getDeviceStats());
  const [analytics] = createResource(() => api.getDeviceAnalytics());
  const onlinePercent = createMemo(() => {
    const total = stats()?.total ?? 0;
    return total > 0 ? Math.round(((stats()?.online ?? 0) / total) * 1000) / 10 : 0;
  });

  return (
    <div class="space-y-4">
      <PageHeader
        title="Network overview"
        description="Current fleet reachability, contact recency, health thresholds, and hardware mix."
        status={<span class="inline-flex items-center gap-2 text-[11px] text-muted"><span class="w-1.5 h-1.5 bg-emerald-500" />Live database view</span>}
      />

      <Show when={(analytics()?.total ?? 0) > (analytics()?.sampled ?? 0)}>
        <div class="px-4 py-3 border border-amber-500/30 text-amber-400 text-xs">
          Analytics use the most recent {analytics()?.sampled.toLocaleString()} of {analytics()?.total.toLocaleString()} devices; fleet totals remain exact.
        </div>
      </Show>

      <Show when={!stats.error} fallback={<div class="px-4 py-3 border border-red-500/30 text-red-400 text-xs">Unable to load fleet telemetry. Verify the API and database connection.</div>}>
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
