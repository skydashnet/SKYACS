import type { JSX, ParentComponent } from 'solid-js';
import { AlertTriangle, DatabaseZap } from 'lucide-solid';

interface EmptyStateProps {
  title: string;
  description: string;
  icon?: JSX.Element;
  action?: JSX.Element;
  compact?: boolean;
}

export const EmptyState: ParentComponent<EmptyStateProps> = (props) => (
  <div class={`resource-state ${props.compact ? 'is-compact' : ''}`}>
    <span class="resource-state-icon" aria-hidden="true">{props.icon ?? <DatabaseZap size={22} />}</span>
    <strong>{props.title}</strong>
    <p>{props.description}</p>
    {props.action && <div class="resource-state-actions">{props.action}</div>}
    {props.children}
  </div>
);

interface ResourceErrorProps {
  title: string;
  description: string;
  onRetry?: () => void;
}

export const ResourceError = (props: ResourceErrorProps) => (
  <div class="resource-state is-error" role="alert">
    <span class="resource-state-icon" aria-hidden="true"><AlertTriangle size={22} /></span>
    <strong>{props.title}</strong>
    <p>{props.description}</p>
    {props.onRetry && <div class="resource-state-actions"><button type="button" class="btn btn-secondary" onClick={props.onRetry}>Retry request</button></div>}
  </div>
);
