import {validateRuntimeConfig} from './policy.mjs';
import {serverCompose} from '../plan.mjs';
import {renderNginx} from '../../runtime/nginx.mjs';

export function candidateCompose(base,images) {
 validateRuntimeConfig(base,images);
 const result=serverCompose(base,{publicTLS:true,publicAccess:true});
 result.services.bff.image=result.services['audit-maintenance'].image=images.bff;
 result.services.nginx.image=images.web;
 return result;
}

export function publicNginx(source) {
 // A pinned receiver only accepts its reviewed rules, not arbitrary uploaded
 // Nginx programs that happen to contain three security-related substrings.
 if(source.replaceAll('\r\n','\n')!==renderNginx())throw Error('Ingress rules require reviewed receiver upgrade');
 return renderNginx({domain:'weave.hubujiu.site',redirect:true});
}
