export type User={id:string;account:string};
export type Permission={code:string;name:string;category:'system'|'application';appId:string|null;enabled?:boolean;sources?:{identityId:string;identityName:string;templateId?:string;templateName?:string}[]};
export type Definition={id:string;name:string;description:string;version:number;permissionCodes:string[];templateIds?:string[];affectedMembers:number;affectedIdentities:number};
export type Access={user:User;bootstrapAdmin:boolean;personnelManage:boolean;identities:Definition[];permissions:Permission[];applications:Permission[]};
export type Department={id:string;parentId:string|null;name:string;isRoot:boolean;version:number;memberCount:number;childrenCount:number};
export type Member=User&{status:string;bootstrapAdmin:boolean;version:number;departmentIds:string[];identityIds:string[];departments:Department[];identities:Definition[];permissions:Permission[]};
export type Activity={id:string;occurredAt:string;actorAccount:string;action:string;objectType:string|null;objectId:string|null;summary:Record<string,unknown>|null;outcome:string};
export type PageData<T>={items:T[];total:number;page:number;pageSize:number};
