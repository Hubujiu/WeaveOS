import test from 'node:test';
import assert from 'node:assert/strict';
import {verifyPackageResult} from './verify-package.mjs';
test('compatible package still must finish successfully',()=>{
 assert.equal(verifyPackageResult({status:0,stderr:''},true),'packaged');
 assert.throws(()=>verifyPackageResult({status:1,stderr:'build failure'},true));
});
test('unapproved compatibility requires original receiver policy rejection',()=>{
 assert.equal(verifyPackageResult({status:1,stderr:'Error: Release identity/compatibility rejected\n at validateCompatibility'},false),'promotion-blocked');
 for(const result of [{status:0,stderr:''},{status:1,stderr:'Docker unavailable'},{status:null,stderr:'Release identity/compatibility rejected'},{status:2,stderr:'Release identity/compatibility rejected'}])assert.throws(()=>verifyPackageResult(result,false));
 for(const value of [null,undefined,'false',0])assert.throws(()=>verifyPackageResult({status:0,stderr:''},value));
});
