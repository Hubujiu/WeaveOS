import type { ReactNode } from 'react';

export type Application = { id: string; name: string; ownerUserId: string; policyRevision: number };
export type ApplicationAccess = { appId: string; policyRevision: number; canEnter: boolean; menus: { resourceKind: 'application'; resourceId: string }[] };
export type ApplicationList = { items: Application[] };
export type PolicyResult = { id: string; policyRevision: number };
export type PermissionGroup = PolicyResult & { name: string; enabled: boolean };
export type MenuGrant = { resourceKind: 'application'; resourceId: string; action: 'menu.enter'; rowScope: 'all'; fields: [] };
export type ApplicationOperation<T> = { operationId: string; status: 'confirmed'; httpStatus: 200 | 201; location: string; result: T };

// Future contracts supply personal pins and actual form tabs. This slice never
// persists them locally or synthesizes form entries.
export type ApplicationTab = { application: Application; pinned: boolean };
export type PersonalNavigation = { userId: string; pinnedApplicationIds: readonly string[] };
export type FormTab = { id: string; label: string; dirty: boolean; content: ReactNode };
export type ApplicationFormSlots = { tabs: readonly FormTab[]; activeId?: string; select: (id: string) => void };

