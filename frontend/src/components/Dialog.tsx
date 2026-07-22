import { createUniqueId, onCleanup, onMount, type JSX, type ParentComponent } from 'solid-js';
import { Portal } from 'solid-js/web';
import { X } from 'lucide-solid';

interface DialogProps {
  title: string;
  description?: string;
  actions?: JSX.Element;
  size?: 'small' | 'medium' | 'large';
  closeLabel?: string;
  closeOnBackdrop?: boolean;
  onClose: () => void;
}

const focusableSelector = [
  'button:not([disabled])',
  '[href]',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

const Dialog: ParentComponent<DialogProps> = (props) => {
  const titleId = createUniqueId();
  const descriptionId = createUniqueId();
  let panel: HTMLDivElement | undefined;
  const previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null;

  const focusableElements = () => Array.from(panel?.querySelectorAll<HTMLElement>(focusableSelector) ?? []);

  const handleKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      props.onClose();
      return;
    }
    if (event.key !== 'Tab') return;
    const elements = focusableElements();
    if (elements.length === 0) {
      event.preventDefault();
      panel?.focus();
      return;
    }
    const first = elements[0];
    const last = elements[elements.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  };

  onMount(() => {
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    document.addEventListener('keydown', handleKeyDown);
    queueMicrotask(() => {
      const preferred = panel?.querySelector<HTMLElement>('[autofocus]');
      (preferred ?? focusableElements()[0] ?? panel)?.focus();
    });
    onCleanup(() => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener('keydown', handleKeyDown);
      previouslyFocused?.focus();
    });
  });

  return (
    <Portal>
      <div
        class="dialog-backdrop"
        onMouseDown={(event) => {
          if (props.closeOnBackdrop !== false && event.target === event.currentTarget) props.onClose();
        }}
      >
        <div
          ref={panel}
          class={`dialog-panel dialog-${props.size ?? 'medium'}`}
          role="dialog"
          aria-modal="true"
          aria-labelledby={titleId}
          aria-describedby={props.description ? descriptionId : undefined}
          tabindex="-1"
        >
          <header class="dialog-header">
            <div>
              <h2 id={titleId}>{props.title}</h2>
              {props.description && <p id={descriptionId}>{props.description}</p>}
            </div>
            <button type="button" class="icon-button" onClick={props.onClose} aria-label={props.closeLabel ?? 'Close dialog'}>
              <X size={17} />
            </button>
          </header>
          <div class="dialog-body">{props.children}</div>
          {props.actions && <footer class="dialog-actions">{props.actions}</footer>}
        </div>
      </div>
    </Portal>
  );
};

export default Dialog;
