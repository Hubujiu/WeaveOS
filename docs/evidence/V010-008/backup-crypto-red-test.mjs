import test from 'node:test';
import assert from 'node:assert/strict';
import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto';
import { seal, open } from './backup-crypto.mjs';
const header = Buffer.from('WEAVEOS_BACKUP_V1\n');
// ADR004 encrypted backups; use independent Node/OpenSSL AES-GCM interoperability,
// not the implementation's own output as the sole expected result.
test('encrypted dump is authenticated AES256-GCM readable independently', () => {
 const key=randomBytes(32), plain=Buffer.from('synthetic pg_dump including sensitive fixture fields');
 const backup=seal(plain,key);
 assert.ok(backup.subarray(0,header.length).equals(header),'versioned backup header required');
 assert.equal(backup.includes(plain),false,'plaintext must not appear in persisted ciphertext');
 const iv=backup.subarray(header.length,header.length+12),tag=backup.subarray(header.length+12,header.length+28);
 const decipher=createDecipheriv('aes-256-gcm',key,iv,{authTagLength:16});decipher.setAAD(header);decipher.setAuthTag(tag);
 assert.deepEqual(Buffer.concat([decipher.update(backup.subarray(header.length+28)),decipher.final()]),plain);
 assert.notDeepEqual(seal(plain,key),backup,'each backup needs an independent fresh nonce');
});
test('restore opens a separately created authenticated dump', () => {
 const key=randomBytes(32),iv=randomBytes(12),plain=Buffer.from('independent fixture');
 const cipher=createCipheriv('aes-256-gcm',key,iv,{authTagLength:16});cipher.setAAD(header);
 const ciphertext=Buffer.concat([cipher.update(plain),cipher.final()]);
 assert.deepEqual(open(Buffer.concat([header,iv,cipher.getAuthTag(),ciphertext]),key),plain);
});
test('wrong key, truncated bytes, modified tag/header/ciphertext never produce a restore', () => {
 const key=randomBytes(32),backup=seal(Buffer.from('synthetic dump'),key);
 assert.throws(()=>open(backup,randomBytes(32)));
 assert.throws(()=>open(backup.subarray(0,10),key));
 for(const index of [0,header.length+12,backup.length-1]){const broken=Buffer.from(backup);broken[index]^=1;assert.throws(()=>open(broken,key));}
 assert.throws(()=>seal(Buffer.from('dump'),Buffer.alloc(16)));
});
