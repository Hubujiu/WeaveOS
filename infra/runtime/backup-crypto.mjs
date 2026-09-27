import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto';
const header=Buffer.from('WEAVEOS_BACKUP_V1\n');
function validKey(key){if(!Buffer.isBuffer(key)||key.length!==32)throw new Error('Backup key must be 32 random bytes');}
export function seal(plain,key){
 validKey(key);if(!Buffer.isBuffer(plain))throw new Error('Backup input must be bytes');
 const nonce=randomBytes(12),cipher=createCipheriv('aes-256-gcm',key,nonce,{authTagLength:16});cipher.setAAD(header);
 const ciphertext=Buffer.concat([cipher.update(plain),cipher.final()]);
 return Buffer.concat([header,nonce,cipher.getAuthTag(),ciphertext]);
}
export function open(encrypted,key){
 validKey(key);
 if(!Buffer.isBuffer(encrypted)||encrypted.length<header.length+28||!encrypted.subarray(0,header.length).equals(header))throw new Error('Invalid backup format');
 const decipher=createDecipheriv('aes-256-gcm',key,encrypted.subarray(header.length,header.length+12),{authTagLength:16});
 decipher.setAAD(header);decipher.setAuthTag(encrypted.subarray(header.length+12,header.length+28));
 // Never return partial plaintext until final authenticates the complete dump.
 try{return Buffer.concat([decipher.update(encrypted.subarray(header.length+28)),decipher.final()]);}
 catch{throw new Error('Backup authentication failed');}
}
