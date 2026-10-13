import {test} from 'node:test';
import assert from 'node:assert/strict';
import {safeNetworkFailure} from '../acceptance/browser-diagnostics.mjs';

test('test diagnostics distinguish known transport failures without retaining engine text',()=>{
 for(const [raw,expected] of [['Load failed','load-failed'],['cancelled','cancelled'],['net::ERR_ABORTED','cancelled'],['net::ERR_CONNECTION_RESET','connection-reset'],['net::ERR_CONNECTION_REFUSED','connection-refused'],['Timeout was reached','timeout'],['net::ERR_TIMED_OUT','timeout']])assert.equal(safeNetworkFailure(raw),expected);
});
test('test diagnostics never expose arbitrary errors or appended private values',()=>{
 for(const raw of [undefined,null,'','arbitrary private value','Load failed https://private.invalid/?session=synthetic-private-value','net::ERR_CONNECTION_RESET synthetic-private-value'])assert.equal(safeNetworkFailure(raw),'unknown');
});
