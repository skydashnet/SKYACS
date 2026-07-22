import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For } from 'solid-js';
import { Upload, Trash2, HardDrive, Package } from 'lucide-solid';
import { api, type Firmware } from '../lib/api';
import { useAuth } from '../lib/auth';
import PageHeader from '../components/PageHeader';
import { useFeedback } from '../components/Feedback';
import { EmptyState, ResourceError } from '../components/ResourceState';

const Firmwares: Component = () => {
  const { isFullAccess } = useAuth();
  const { confirm, notify } = useFeedback();
  const [firmwares, { refetch }] = createResource(() => api.getFirmwares());
  const [uploading, setUploading] = createSignal(false);
  const [validation, setValidation] = createSignal<{ file?: string; version?: string }>({});

  const [formData, setFormData] = createSignal({
    version: '',
    manufacturer: '',
    product_class: '',
    description: '',
  });

  let fileInputRef: HTMLInputElement | undefined;

  const handleUpload = async () => {
    const file = fileInputRef?.files?.[0];
    const errors: { file?: string; version?: string } = {};
    if (!file) {
      errors.file = 'Select a firmware artifact before starting the upload.';
    }

    const data = formData();
    if (!data.version.trim()) {
      errors.version = 'Enter the vendor firmware version exactly as it should appear in deployment records.';
    }
    setValidation(errors);
    if (Object.keys(errors).length > 0 || !file) return;

    setUploading(true);
    try {
      await api.uploadFirmware(file, data.version, data.manufacturer, data.product_class, data.description);
      notify({ tone: 'success', title: 'Firmware artifact uploaded', message: `${file.name} is available for controlled CPE deployment.` });
      await refetch();
      setFormData({ version: '', manufacturer: '', product_class: '', description: '' });
      setValidation({});
      if (fileInputRef) fileInputRef.value = '';
    } catch (err) {
      notify({ tone: 'error', title: 'Firmware upload failed', message: 'The artifact was not added to the library. Correct the reported issue and retry.', detail: (err as Error).message, persistent: true });
    } finally {
      setUploading(false);
    }
  };

  const handleDelete = async (id: number, filename: string) => {
    if (!await confirm({ title: `Delete ${filename}?`, description: 'The artifact will no longer be available for new firmware tasks. Existing task records remain in the audit history.', confirmLabel: 'Delete firmware', tone: 'danger' })) return;
    try {
      await api.deleteFirmware(id);
      notify({ tone: 'success', title: 'Firmware artifact deleted', message: filename });
      await refetch();
    } catch (err) {
      notify({ tone: 'error', title: 'Could not delete firmware', message: 'The artifact remains available. Retry after checking active deployment references.', detail: (err as Error).message, persistent: true });
    }
  };

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(2) + ' MB';
  };

  const formatDate = (dateStr: string) => new Date(dateStr).toLocaleString('id-ID');

  return (
    <div class="space-y-6">
      <PageHeader title="Firmware library" description="Validated artifacts ready for controlled CPE deployment." />

      <Show when={isFullAccess()}><div class="card p-5">
        <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
          <Upload size={14} />
          Upload firmware artifact
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label for="firmware-file" class="block text-xs text-muted mb-1.5">Firmware file</label>
            <input
              id="firmware-file"
              ref={fileInputRef}
              type="file"
              accept=".bin,.img,.tar,.gz,.zip"
              class="w-full px-3 py-2 bg-surface border border-default rounded-[3px] text-sm text-secondary file:mr-3 file:py-1 file:px-3 file:rounded-[2px] file:border-0 file:bg-sky-600 file:text-white file:text-xs file:cursor-pointer"
              aria-invalid={Boolean(validation().file)}
              aria-describedby={validation().file ? 'firmware-file-error' : undefined}
            />
            <Show when={validation().file}><p id="firmware-file-error" class="field-error">{validation().file}</p></Show>
          </div>
          <div>
            <label for="firmware-version" class="block text-xs text-muted mb-1.5">Firmware version <span aria-hidden="true">*</span></label>
            <input
              id="firmware-version"
              type="text"
              value={formData().version}
              onInput={(e) => setFormData({ ...formData(), version: e.currentTarget.value })}
              placeholder="e.g. V5R019C00S100"
              class="input"
              required
              aria-invalid={Boolean(validation().version)}
              aria-describedby={validation().version ? 'firmware-version-error' : undefined}
            />
            <Show when={validation().version}><p id="firmware-version-error" class="field-error">{validation().version}</p></Show>
          </div>
          <div>
            <label for="firmware-manufacturer" class="block text-xs text-muted mb-1.5">Manufacturer</label>
            <input
              id="firmware-manufacturer"
              type="text"
              value={formData().manufacturer}
              onInput={(e) => setFormData({ ...formData(), manufacturer: e.currentTarget.value })}
              placeholder="e.g. Huawei"
              class="input"
            />
          </div>
          <div>
            <label for="firmware-product-class" class="block text-xs text-muted mb-1.5">Product class</label>
            <input
              id="firmware-product-class"
              type="text"
              value={formData().product_class}
              onInput={(e) => setFormData({ ...formData(), product_class: e.currentTarget.value })}
              placeholder="e.g. HG8245W5"
              class="input"
            />
          </div>
          <div class="md:col-span-2">
            <label for="firmware-description" class="block text-xs text-muted mb-1.5">Deployment notes</label>
            <textarea
              id="firmware-description"
              value={formData().description}
              onInput={(e) => setFormData({ ...formData(), description: e.currentTarget.value })}
              placeholder="Optional compatibility or rollout notes"
              rows={2}
              class="input resize-none"
            />
          </div>
        </div>
        <button
          onClick={handleUpload}
          disabled={uploading()}
          class="btn btn-primary mt-4"
        >
          <Upload size={14} />
          {uploading() ? 'Uploading artifact…' : 'Upload firmware'}
        </button>
      </div></Show>

      <div class="card overflow-hidden">
        <div class="p-5 border-b border-subtle">
          <h2 class="text-sm font-medium text-secondary flex items-center gap-2">
            <Package size={14} />
            Firmware Library ({firmwares()?.length || 0})
          </h2>
        </div>
        <Show when={firmwares.loading}><div class="p-4 space-y-3" aria-label="Loading firmware library"><div class="skeleton h-8 w-full" /><div class="skeleton h-8 w-4/5" /></div></Show>
        <Show when={firmwares.error}><ResourceError title="Firmware library is unavailable" description="SKYACS could not retrieve the artifact inventory. No firmware data was changed." onRetry={() => refetch()} /></Show>
        <Show when={!firmwares.loading && !firmwares.error && (firmwares()?.length || 0) > 0} fallback={!firmwares.loading && !firmwares.error ?
          <EmptyState icon={<HardDrive size={22} />} title="No firmware artifacts are stored" description={isFullAccess() ? 'Upload a vendor firmware file with an exact version and compatibility scope before creating a deployment task.' : 'A full-access operator must upload and validate an artifact before it can be selected for deployment.'} /> : undefined
        }>
          <div class="overflow-x-auto"><table class="data-table w-full min-w-[720px]">
            <thead>
              <tr class="border-b border-subtle">
                <th class="px-4 py-3 text-left text-xs font-medium text-muted">Filename</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted">Version</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted">Manufacturer</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted">Size</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted">Uploaded</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted">Actions</th>
              </tr>
            </thead>
            <tbody>
              <For each={firmwares()}>
                {(fw: Firmware) => (
                  <tr class="border-t border-subtle/50 hover:bg-elevated/30 transition-fast">
                    <td class="px-4 py-3 text-primary font-mono text-sm">{fw.filename}</td>
                    <td class="px-4 py-3 font-mono text-xs text-secondary">{fw.version}</td>
                    <td class="px-4 py-3 text-secondary text-sm">{fw.manufacturer || '-'}</td>
                    <td class="px-4 py-3 text-secondary text-sm">{formatSize(fw.file_size)}</td>
                    <td class="px-4 py-3 text-muted text-xs">{formatDate(fw.created_at)}</td>
                    <td class="px-4 py-3">
                      <Show when={isFullAccess()}><button
                        onClick={() => handleDelete(fw.id, fw.filename)}
                        class="text-rose-400 hover:text-rose-300 text-sm flex items-center gap-1 transition-fast"
                      >
                        <Trash2 size={12} />
                        Delete
                      </button></Show>
                    </td>
                  </tr>
                )}
              </For>
            </tbody>
          </table></div>
        </Show>
      </div>

      <p class="text-muted text-xs">
        To deploy an artifact, open a CPE record and create a controlled firmware download task.
      </p>
    </div>
  );
};

export default Firmwares;
