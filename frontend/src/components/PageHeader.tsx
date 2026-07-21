import type { JSX, ParentComponent } from 'solid-js';

interface PageHeaderProps {
  title: string;
  description: string;
  status?: JSX.Element;
}

const PageHeader: ParentComponent<PageHeaderProps> = (props) => (
  <header class="page-header">
    <div class="page-header-copy">
      <h1>{props.title}</h1>
      <p>{props.description}</p>
    </div>
    {(props.status || props.children) && (
      <div class="page-header-tools">
        {props.status}
        {props.children}
      </div>
    )}
  </header>
);

export default PageHeader;
