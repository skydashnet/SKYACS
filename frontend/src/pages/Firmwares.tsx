import type { Component } from 'solid-js';
import { createResource, createSignal, Show, For } from 'solid-js';
import { Upload, Trash2, HardDrive, Package } from 'lucide-solid';
import { api, type Firmware } from '../lib/api';
import { useAuth } from '../lib/auth';

const Firmwares: Component = () => {
  const { isFullAccess } = useAuth();
  const [firmwares, { refetch }] = createResource(() => api.getFirmwares());
  const [uploading, setUploading] = createSignal(false);
  const [message, setMessage] = createSignal<{ type: 'success' | 'error'; text: string } | null>(null);

  const [formData, setFormData] = createSignal({
    version: '',
    manufacturer: '',
    product_class: '',
    description: '',
  });

  let fileInputRef: HTMLInputElement | undefined;

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 5000);
  };

  const handleUpload = async () => {
    const file = fileInputRef?.files?.[0];
    if (!file) {
      showMessage('error', 'Pilih file firmware terlebih dahulu');
      return;
    }

    const data = formData();
    if (!data.version.trim()) {
      showMessage('error', 'Version wajib diisi');
      return;
    }

    setUploading(true);
    try {
      await api.uploadFirmware(file, data.version, data.manufacturer, data.product_class, data.description);
      showMessage('success', 'Firmware berhasil diupload');
      refetch();
      setFormData({ version: '', manufacturer: '', product_class: '', description: '' });
      if (fileInputRef) fileInputRef.value = '';
    } catch (err) {
      showMessage('error', 'Upload gagal: ' + (err as Error).message);
    }
    setUploading(false);
  };

  const handleDelete = async (id: number, filename: string) => {
    if (!confirm(`Hapus firmware "${filename}"?`)) return;
    try {
      await api.deleteFirmware(id);
      showMessage('success', 'Firmware dihapus');
      refetch();
    } catch (err) {
      showMessage('error', 'Gagal hapus: ' + (err as Error).message);
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
      <div><p class="text-[10px] uppercase tracking-[.12em] text-sky-500 font-semibold">Image lifecycle</p><h1 class="text-xl font-semibold text-primary mt-1">Firmware library</h1><p class="text-xs text-muted mt-1">Validated artifacts ready for controlled CPE deployment.</p></div>

      <Show when={message()}>
        <div class={`p-3 rounded-md text-sm ${message()?.type === 'success' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'}`}>
          {message()?.text}
        </div>
      </Show>

      {/* Upload Form */}
      <Show when={isFullAccess()}><div class="card p-5">
        <h2 class="text-sm font-medium text-secondary mb-4 flex items-center gap-2">
          <Upload size={14} />
          Upload Firmware Baru
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label class="block text-xs text-muted mb-1.5">File Firmware</label>
            <input
              ref={fileInputRef}
              type="file"
              accept=".bin,.img,.tar,.gz,.zip"
              class="w-full px-3 py-2 bg-surface border border-default rounded-[3px] text-sm text-secondary file:mr-3 file:py-1 file:px-3 file:rounded-[2px] file:border-0 file:bg-sky-600 file:text-white file:text-xs file:cursor-pointer"
            />
          </div>
          <div>
            <label class="block text-xs text-muted mb-1.5">Version *</label>
            <input
              type="text"
              value={formData().version}
              onInput={(e) => setFormData({ ...formData(), version: e.currentTarget.value })}
              placeholder="e.g. V5R019C00S100"
              class="input"
            />
          </div>
          <div>
            <label class="block text-xs text-muted mb-1.5">Manufacturer</label>
            <input
              type="text"
              value={formData().manufacturer}
              onInput={(e) => setFormData({ ...formData(), manufacturer: e.currentTarget.value })}
              placeholder="e.g. Huawei"
              class="input"
            />
          </div>
          <div>
            <label class="block text-xs text-muted mb-1.5">Product Class</label>
            <input
              type="text"
              value={formData().product_class}
              onInput={(e) => setFormData({ ...formData(), product_class: e.currentTarget.value })}
              placeholder="e.g. HG8245W5"
              class="input"
            />
          </div>
          <div class="md:col-span-2">
            <label class="block text-xs text-muted mb-1.5">Description</label>
            <textarea
              value={formData().description}
              onInput={(e) => setFormData({ ...formData(), description: e.currentTarget.value })}
              placeholder="Optional description"
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
          {uploading() ? 'Uploading...' : 'Upload Firmware'}
        </button>
      </div></Show>

      {/* Firmware List */}
      <div class="card overflow-hidden">
        <div class="p-5 border-b border-subtle">
          <h2 class="text-sm font-medium text-secondary flex items-center gap-2">
            <Package size={14} />
            Firmware Library ({firmwares()?.length || 0})
          </h2>
        </div>
        <Show when={(firmwares()?.length || 0) > 0} fallback={
          <div class="p-12 text-center">
            <HardDrive size={32} class="mx-auto text-muted opacity-50 mb-2" />
            <p class="text-muted text-sm">Belum ada firmware yang diupload</p>
          </div>
        }>
          <table class="w-full">
            <thead>
              <tr class="border-b border-subtle">
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide">Filename</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide">Version</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide">Manufacturer</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide">Size</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide">Uploaded</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-muted uppercase tracking-wide">Actions</th>
              </tr>
            </thead>
            <tbody>
              <For each={firmwares()}>
                {(fw: Firmware) => (
                  <tr class="border-t border-subtle/50 hover:bg-elevated/30 transition-fast">
                    <td class="px-4 py-3 text-primary font-mono text-sm">{fw.filename}</td>
                    <td class="px-4 py-3"><span class="badge badge-success">{fw.version}</span></td>
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
          </table>
        </Show>
      </div>

      <p class="text-muted text-xs">
        Untuk push firmware ke device, buka halaman Device Detail dan gunakan tombol "Download Firmware".
      </p>
    </div>
  );
};

export default Firmwares;
