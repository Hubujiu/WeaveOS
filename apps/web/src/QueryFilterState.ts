import type { EventFilterGroup, MemberFilterGroup } from './query-contracts';

export type QueryView = 'members' | 'events';
export type QueryFilter<V extends QueryView> = V extends 'members' ? MemberFilterGroup : EventFilterGroup;
export type FilterIssue = { path: string; message: string };
export type FilterValidation<V extends QueryView> = { filter?: QueryFilter<V>; issues: FilterIssue[] };

// RED-only loading declaration. No validation behavior until the observed RED.
export function validateQueryFilter<V extends QueryView>(_view: V, _input: unknown): FilterValidation<V> {
  return { issues: [] };
}
