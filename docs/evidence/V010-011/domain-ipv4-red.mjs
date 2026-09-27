import {request as httpsRequest} from 'node:https';
import {existsSync} from 'node:fs';
import {sampleRuntime} from '../runtime/probe.mjs';
import {acmePlan} from './acme.mjs';
export async function domainHealth(path,request=httpsRequest) {
 if(!['ready','live'].includes(path))throw Error('Health operation rejected');
 return new Promise(resolve=>{
  const req=request({hostname:acmePlan().domain,port:19443,servername:acmePlan().domain,path:'/health/'+path,rejectUnauthorized:true,lookup:(_host,_options,callback)=>callback(null,'127.0.0.1',4)},response=>{response.resume();resolve(response.statusCode===200);});
  req.once('error',()=>resolve(false));req.setTimeout(3000,()=>{req.destroy();resolve(false);});req.end();
 });
}
export async function sampleServer(c) {
 const sample=await sampleRuntime(c);
 if(existsSync(c.dir+'/tls/public-trust-ready'))[sample.ready,sample.live]=await Promise.all([domainHealth('ready'),domainHealth('live')]);
 return sample;
}
