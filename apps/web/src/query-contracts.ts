// Q36 wire declarations only. OpenAPI + V010-020-PLAN are the authority.
// Runtime validation, query state and UI are delivered by their separate owners.
export type QueryVersion = string;
export type EqualityOperator = 'eq' | 'neq';
export type OrderingOperator = 'gt' | 'gte' | 'lt' | 'lte';
export type CalendarDateValue = { date: string; timeZone: string };
export type InstantOrDate = string | CalendarDateValue;

export type MemberFilterCondition =
  | { field: 'account'; operator: EqualityOperator; value: string | null }
  | { field: 'status'; operator: EqualityOperator; value: 'active' | 'disabled' | null }
  | { field: 'departmentIds' | 'identityIds'; operator: EqualityOperator; value: string }
  | { field: 'personnelManage'; operator: EqualityOperator; value: boolean | null };

export type ActivityAction =
  | 'DEPARTMENT_CREATED' | 'DEPARTMENT_UPDATED' | 'DEPARTMENT_DELETED'
  | 'IDENTITY_CREATED' | 'IDENTITY_UPDATED' | 'IDENTITY_DELETED'
  | 'TEMPLATE_CREATED' | 'TEMPLATE_UPDATED' | 'TEMPLATE_DELETED'
  | 'MEMBER_IDENTITIES_UPDATED' | 'MEMBER_GROUPS_UPDATED' | 'INVITATION_CREATED';
export type EventFilterCondition =
  | { field: 'actorAccount' | 'object' | 'detail'; operator: EqualityOperator; value: string | null }
  | { field: 'action'; operator: EqualityOperator; value: ActivityAction | null }
  | { field: 'outcome'; operator: EqualityOperator; value: 'success' | 'failure' | 'error' | null }
  | { field: 'occurredAt'; operator: EqualityOperator; value: InstantOrDate | null }
  | { field: 'occurredAt'; operator: OrderingOperator; value: InstantOrDate };

type Group<T> = { operator: 'and' | 'or'; children: [T, ...T[]] };
// Root is group level 1. Global <=20 leaves and <=16384 canonical UTF-8 bytes
// still require runtime validation; empty input is absence of filter, not {}.
export type FilterGroup<Leaf> = Group<Leaf | Group<Leaf | Group<Leaf>>>;
export type MemberFilterGroup = FilterGroup<MemberFilterCondition>;
export type EventFilterGroup = FilterGroup<EventFilterCondition>;
export type QuerySort = { key: 'occurredAt'; direction: 'asc' | 'desc' };
export type QueryRange = { from: string; to: string };
export type ActivityDisplay = { action: string; object: string; detail: string; outcome: string };

export type QueryPage<Row, Sort extends QuerySort | null = QuerySort | null> = {
  items: Row[]; total: number; page: number; pageSize: number;
  queryVersion: QueryVersion; sort: Sort;
};
export type MembersQueryPage<Row> = QueryPage<Row, null>;
export type EventsQueryPage<Row> = QueryPage<Row & { display: ActivityDisplay }> & { range: QueryRange };
export type MemberQueryParameters = {
  page?: number; pageSize?: number; search?: string;
  departmentId?: string; identityId?: string;
  filter?: MemberFilterGroup; queryVersion?: QueryVersion;
};
export type EventQueryParameters = {
  page?: number; pageSize?: number; search?: string; action?: ActivityAction;
  from?: string; to?: string; filter?: EventFilterGroup; queryVersion?: QueryVersion;
} & ({ sortBy?: never; sortDirection?: never } | { sortBy: 'occurredAt'; sortDirection: 'asc' | 'desc' });

export type DraftReference = { id: string; version: number };
// Member/department business requests require queryVersion. Definition cards
// retain their object-version workflow; all supported editors may pass draftRef.
export type QueryWriteGuard = { queryVersion: QueryVersion; draftRef?: DraftReference };
export type DraftAcknowledgement = { draftRef?: DraftReference };
export type DraftPayloadByKind = {
  'member-identities': { identityIds: string[] };
  'member-groups': { operation: 'add' | 'remove' | 'move'; departmentId: string | null; sourceDepartmentId: string | null };
  department: { name: string; parentId: string | null };
  identity: { name: string; description: string; templateIds: string[]; permissionCodes: string[] };
  template: { name: string; description: string; permissionCodes: string[] };
};
export type DraftKind = keyof DraftPayloadByKind;
type DraftTarget<Kind extends DraftKind> = Kind extends 'member-identities' | 'member-groups'
  ? { targetId: string; baseVersion: number }
  : { targetId: string; baseVersion: number } | { targetId: null; baseVersion: null };
export type DraftCreateInput = {
  [Kind in DraftKind]: { kind: Kind; payload: DraftPayloadByKind[Kind] } & DraftTarget<Kind>
}[DraftKind];
// Server validates payload against the stored, immutable kind on PUT.
export type DraftUpdateInput<Kind extends DraftKind = DraftKind> = { version: number; payload: DraftPayloadByKind[Kind] };
export type PersonnelDraftSummary = {
  id: string; kind: DraftKind; targetId: string | null; baseVersion: number | null;
  version: number; createdAt: string; updatedAt: string;
};
export type PersonnelDraft = PersonnelDraftSummary & DraftCreateInput;
export type PersonnelDraftList = { items: PersonnelDraftSummary[] };
export type QueryErrorCode = 'COMMON_QUERY_CHANGED' | 'COMMON_QUERY_CONTEXT_EXPIRED';
export type DraftErrorCode = 'PERSONNEL_DRAFT_CONFLICT' | 'PERSONNEL_DRAFT_LIMIT_REACHED';
export type QueryBusyEnvelope = {
  code: 'COMMON_SERVICE_UNAVAILABLE'; message: string; data: null;
  meta: { requestId: string; reason: 'QUERY_BUSY' };
};
