import { createContext, createSignal, For, Show, useContext, type ParentComponent } from 'solid-js';
import { Portal } from 'solid-js/web';
import { AlertTriangle, CheckCircle2, Info, X } from 'lucide-solid';
import Dialog from './Dialog';

type NoticeTone = 'success' | 'error' | 'info';

interface NoticeInput {
  tone: NoticeTone;
  title: string;
  message?: string;
  detail?: string;
  persistent?: boolean;
}

interface Notice extends NoticeInput {
  id: number;
}

interface ConfirmOptions {
  title: string;
  description: string;
  confirmLabel: string;
  cancelLabel?: string;
  tone?: 'danger' | 'primary';
}

interface FeedbackContextValue {
  notify: (notice: NoticeInput) => void;
  confirm: (options: ConfirmOptions) => Promise<boolean>;
}

const FeedbackContext = createContext<FeedbackContextValue>();

export const FeedbackProvider: ParentComponent = (props) => {
  const [notices, setNotices] = createSignal<Notice[]>([]);
  const [confirmation, setConfirmation] = createSignal<(ConfirmOptions & { resolve: (accepted: boolean) => void }) | null>(null);
  let nextNoticeId = 1;

  const dismiss = (id: number) => setNotices((current) => current.filter((notice) => notice.id !== id));
  const notify = (input: NoticeInput) => {
    const notice = { ...input, id: nextNoticeId++ };
    setNotices((current) => [...current, notice]);
    if (!input.persistent && input.tone !== 'error') window.setTimeout(() => dismiss(notice.id), 5000);
  };
  const confirm = (options: ConfirmOptions) => new Promise<boolean>((resolve) => {
    confirmation()?.resolve(false);
    setConfirmation({ ...options, resolve });
  });
  const resolveConfirmation = (accepted: boolean) => {
    const current = confirmation();
    setConfirmation(null);
    current?.resolve(accepted);
  };

  return (
    <FeedbackContext.Provider value={{ notify, confirm }}>
      {props.children}
      <Portal>
        <section class="notice-stack" aria-label="System notifications" aria-live="polite">
          <For each={notices()}>{(notice) => (
            <article class={`notice notice-${notice.tone}`} role={notice.tone === 'error' ? 'alert' : 'status'}>
              <span class="notice-icon" aria-hidden="true">
                {notice.tone === 'success' ? <CheckCircle2 size={17} /> : notice.tone === 'error' ? <AlertTriangle size={17} /> : <Info size={17} />}
              </span>
              <div class="notice-copy">
                <strong>{notice.title}</strong>
                <Show when={notice.message}><p>{notice.message}</p></Show>
                <Show when={notice.detail}><details><summary>Technical details</summary><pre>{notice.detail}</pre></details></Show>
              </div>
              <button type="button" class="icon-button" onClick={() => dismiss(notice.id)} aria-label={`Dismiss ${notice.title}`}><X size={15} /></button>
            </article>
          )}</For>
        </section>
      </Portal>
      <Show when={confirmation()}>{(current) => (
        <Dialog
          title={current().title}
          description={current().description}
          size="small"
          onClose={() => resolveConfirmation(false)}
          actions={<>
            <button type="button" class="btn btn-secondary" onClick={() => resolveConfirmation(false)}>{current().cancelLabel ?? 'Cancel'}</button>
            <button type="button" class={`btn ${current().tone === 'danger' ? 'btn-danger' : 'btn-primary'}`} onClick={() => resolveConfirmation(true)} autofocus>{current().confirmLabel}</button>
          </>}
        />
      )}</Show>
    </FeedbackContext.Provider>
  );
};

export const useFeedback = () => {
  const context = useContext(FeedbackContext);
  if (!context) throw new Error('useFeedback must be used within FeedbackProvider');
  return context;
};
