// Q16: a single explicitly authorized DNS name, provider and CA.
export function acmePlan() {
 const root='/opt/weaveos-v010',domain='weave.hubujiu.site';
 const common=['--home',root+'/acme-client','--config-home',root+'/acme-state','--cert-home',root+'/acme-state'];
 const ca=['--server','https://acme-v02.api.letsencrypt.org/directory'];
 return {
  domain,origin:`https://${domain}:19443`,source:{version:'3.1.6'},
  credentials:root+'/secrets/acme.env',
  stagedCertificate:root+'/acme-stage/cert.pem',stagedKey:root+'/acme-stage/key.pem',
  register:['--register-account',...ca,...common],
  issue:['--issue',...ca,'--dns','dns_tencent','-d',domain,'--keylength','ec-256',...common],
  renew:['--cron',...ca,...common],
  install:['--install-cert','-d',domain,'--ecc',...common,'--key-file',root+'/acme-stage/key.pem','--fullchain-file',root+'/acme-stage/cert.pem','--reloadcmd',`/usr/local/bin/node ${root}/infra/server/tls.mjs deploy`]
 };
}
