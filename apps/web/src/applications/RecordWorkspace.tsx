import type {RecordItem,RuntimeView} from './records/contracts';
export type RecordWorkspaceProps={
 actorId:string;appId:string;viewId:string;
 onCreate:(view:RuntimeView,queryVersion?:string)=>void;
 onOpenRecord:(view:RuntimeView,record:RecordItem,queryVersion:string)=>void;
 onUnauthorized:()=>void;onIdentityMismatch:()=>void;
};
// Root-authored declaration-only TDD entrypoint. No product behavior yet.
export function RecordWorkspace(_props:RecordWorkspaceProps){return null;}
