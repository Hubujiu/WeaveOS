import { execFileSync } from 'node:child_process';
import { mkdirSync, existsSync, readFileSync, writeFileSync, copyFileSync, cpSync } from 'node:fs';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
const goImage='golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244';
const debian='debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251';
const nginx='nginx:1.30.5@sha256:b972f831f200b19ef0767938224f9711e74cd783718738cd7405d5cabf75c442';
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
export function exportArtifacts(record,directory){return record;}
export function importArtifacts(file){return JSON.parse(readFileSync(file,'utf8'));}
export function verifyArtifacts(record){
 if(!/^[0-9a-f]{40}$/.test(record?.commit))throw new Error('Source identity invalid');
 for(const name of ['bff','web']){
  const a=record[name];if(!/^sha256:[0-9a-f]{64}$/.test(a?.manifestDigest)||!/^sha256:[0-9a-f]{64}$/.test(a?.imageID))throw new Error('Artifact identity invalid');
  if(sha(readFileSync(a.archive))!==a.archiveSHA256)throw new Error('Artifact archive checksum mismatch');
  const index=JSON.parse(execFileSync('tar',['-xOf',a.archive,'index.json'],{encoding:'utf8'}));
  if(index.manifests.length!==1||index.manifests[0].digest!==a.manifestDigest)throw new Error('OCI manifest descriptor mismatch');
  const manifestBytes=execFileSync('tar',['-xOf',a.archive,`blobs/sha256/${a.manifestDigest.slice(7)}`]);
  if(`sha256:${sha(manifestBytes)}`!==a.manifestDigest)throw new Error('OCI manifest bytes changed');
  const manifest=JSON.parse(manifestBytes),configBytes=execFileSync('tar',['-xOf',a.archive,`blobs/sha256/${manifest.config.digest.slice(7)}`]);
  if(`sha256:${sha(configBytes)}`!==manifest.config.digest)throw new Error('OCI config bytes differ');
  const image=JSON.parse(execFileSync('docker',['image','inspect',a.imageID],{encoding:'utf8'}))[0];
  if(image.Id!==a.imageID||image.Config.Labels['org.opencontainers.image.revision']!==record.commit)throw new Error('Source/Docker image mismatch');
  // containerd IDs reference manifests; classic store IDs reference configs.
  if(a.imageID!==a.manifestDigest&&a.imageID!==manifest.config.digest)throw new Error('Selected image is not this OCI image');
  if(JSON.stringify(image.RootFS.Layers)!==JSON.stringify(JSON.parse(configBytes).rootfs.diff_ids))throw new Error('OCI and Docker filesystem layers differ');
 }
 return true;
}
export function packageImages(o){
 if(!/^[0-9a-f]{40}$/.test(o.commit))throw new Error('Exact source commit required');
 if(existsSync(o.outputDir))throw new Error('Preserve existing artifacts; output cannot be overwritten');
 const out=resolve(o.outputDir);mkdirSync(out,{recursive:true});
 const run=(cmd,args,options={})=>execFileSync(cmd,args,{cwd:o.root,stdio:'inherit',...options});
 const sourceTar=resolve(out,'source.tar');run('git',['archive','--format=tar','--prefix=src/','--output',sourceTar,o.commit]);run('tar',['-xf',sourceTar,'-C',out]);
 const src=resolve(out,'src');
 if(readFileSync(resolve(src,'.go-version'),'utf8').trim()!=='1.27.1'||readFileSync(resolve(src,'.node-version'),'utf8').trim()!=='24.14.0')throw new Error('Recheck locked builder versions');
 run('docker',['run','--rm','--platform','linux/amd64','--mount',`type=bind,src=${src},dst=/repo`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-e','GOFLAGS=-buildvcs=false','-w','/repo/services/bff',goImage,'sh','-ec','mkdir -p /repo/.work/bin; CGO_ENABLED=0 go build -trimpath -o /repo/.work/bin/bff ./cmd/bff; CGO_ENABLED=0 go build -trimpath -o /repo/.work/bin/audit-read ./cmd/audit-read; CGO_ENABLED=0 go build -trimpath -o /repo/.work/bin/audit-maintenance ./cmd/audit-maintenance']);
 run('docker',['run','--rm','--platform','linux/amd64','--mount',`type=bind,src=${src},dst=/repo`,'--mount','type=volume,src=weaveos-v010-artifact-node,dst=/repo/node_modules','--mount','type=volume,src=weaveos-v010-artifact-web-node,dst=/repo/apps/web/node_modules','-e','CI=true','-w','/repo','node:24.14.0-bookworm-slim','sh','-ec','npm install --global pnpm@10.28.2 --ignore-scripts; pnpm install --frozen-lockfile --ignore-scripts --store-dir .work/pnpm-store; pnpm build']);
 const context=resolve(out,'context');mkdirSync(context);
 for(const name of ['bff','audit-read','audit-maintenance'])copyFileSync(resolve(src,'.work/bin',name),resolve(context,name));
 cpSync(resolve(src,'apps/web/dist'),resolve(context,'web'),{recursive:true});
 copyFileSync(resolve(src,'infra/acceptance/nginx.conf'),resolve(context,'nginx.conf'));
 writeFileSync(resolve(context,'bff.Dockerfile'),`FROM ${debian}\nCOPY --chmod=0555 bff audit-read audit-maintenance /app/\nUSER 65532:65532\nCMD ["/app/bff"]\n`);
 writeFileSync(resolve(context,'web.Dockerfile'),`FROM ${nginx}\nCOPY web /usr/share/nginx/html\nCOPY nginx.conf /etc/nginx/nginx.conf\n`);
 const record={commit:o.commit,sourceArchiveSHA256:sha(readFileSync(sourceTar)),builders:{go:goImage,node:'node:24.14.0-bookworm-slim'},classification:'local-development-only',createdAt:new Date().toISOString(),binaries:{}};
 for(const name of ['bff','audit-read','audit-maintenance'])record.binaries[name]=sha(readFileSync(resolve(context,name)));
 for(const name of ['bff','web']){
  const archive=resolve(out,`${name}.oci.tar`),metadata=resolve(out,`${name}.metadata.json`),tag=`weaveos-v010-008-${name}:${o.commit}`;
  run('docker',['buildx','build','--platform','linux/amd64','--provenance=false','--label',`org.opencontainers.image.revision=${o.commit}`,'--tag',tag,'--file',resolve(context,`${name}.Dockerfile`),'--metadata-file',metadata,'--output',`type=oci,dest=${archive}`,context]);
  const meta=JSON.parse(readFileSync(metadata,'utf8'));
  run('docker',['load','--input',archive]);
  const imageID=run('docker',['image','inspect',tag,'--format','{{.Id}}'],{stdio:'pipe',encoding:'utf8'}).trim();
  record[name]={archive,archiveSHA256:sha(readFileSync(archive)),manifestDigest:meta['containerimage.digest'],imageID,tag};
 }
 record.recordFile=resolve(out,'BUILD.json');writeFileSync(record.recordFile,JSON.stringify(record,null,2),{flag:'wx'});return record;
}
