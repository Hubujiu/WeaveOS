import type {StructureCopyPlanRequest} from './copyRequest';
import type {TemplateReferences} from './references';
import type {TemplateCounts} from './receipts';
export type StructureCopyPlan={sourceFingerprint:string;targetStructureVersion:number;targetPolicyRevision:number;manifest:Record<string,unknown>;externalReferences:TemplateReferences};
export type StructureCopyPreflight={valid:true;counts:TemplateCounts;mappedManifest:Record<string,unknown>};
export function validStructureCopyPlan(_value:unknown,_input:StructureCopyPlanRequest):_value is StructureCopyPlan{return false;}
export function validStructureCopyPreflight(_value:unknown,_input:StructureCopyPlanRequest):_value is StructureCopyPreflight{return false;}
