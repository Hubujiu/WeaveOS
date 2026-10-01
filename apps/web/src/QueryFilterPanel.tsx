import type { QueryFilter, QueryView } from './QueryFilterState';

export type FilterChoice = { value: string; label: string };
export type QueryFilterPanelProps<V extends QueryView> = {
  view: V;
  value?: QueryFilter<V>;
  onApply: (filter: QueryFilter<V> | undefined) => void;
  options?: { departmentIds?: readonly FilterChoice[]; identityIds?: readonly FilterChoice[] };
};

// RED-only visible declaration: missing panel behavior must reach assertions.
export function QueryFilterPanel<V extends QueryView>(_props: QueryFilterPanelProps<V>) {
  return <button type="button">自定义筛选</button>;
}
