export type StructureDeletionKind='application'|'directory'|'table'|'form';
export type StructureDeletionResource={kind:'structure-deletion';appId:string;resourceKind:StructureDeletionKind;id:string};
export type StructureDeletionReceipt={operationId:string;appId:string;resourceKind:StructureDeletionKind;id:string;structureVersion:number;deleted:true};
export type StructureDeletionContext={resource:StructureDeletionResource;operationId:string;expectedStructureVersion:number};
export function validStructureDeletionRequest(_resource:unknown,_request:unknown):boolean{return false;}
export function validStructureDeletionReceipt(_value:unknown,_context:StructureDeletionContext):_value is StructureDeletionReceipt{return false;}
export function validStructureDeletionDependencies(_value:unknown,_kind:StructureDeletionKind):_value is {dependencies:string[]}{return false;}
