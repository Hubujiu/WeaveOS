import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdirSync, mkdtempSync, readFileSync, rmSync, unlinkSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {resolve} from 'node:path';
import {Script} from 'node:vm';
import ts from 'typescript';
import {countAPIReport, countBrowserReport} from '../infra/acceptance/result-counts.mjs';

const runners = ['infra/acceptance/run.mjs', 'infra/runtime/run.mjs'];
const named = (node, name) => node && ts.isIdentifier(node) && node.text === name;
const literal = (node, value) => node && ts.isStringLiteral(node) && node.text === value;
const directCall = (node, name) => node && ts.isCallExpression(node) && named(node.expression, name);

// Execute the reviewed repository statement, not an imitation of its counters.
// This tests the extracted publication boundary, not runner reachability,
// imports, report generation, catch/finally, or an arbitrary-code sandbox.
function publicationStatement(source, filename) {
 const parsed = ts.createSourceFile(filename, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
 assert.equal(parsed.parseDiagnostics.length, 0, `${filename}: invalid JavaScript`);
 const candidates = [];
 function visit(node) {
  if (ts.isExpressionStatement(node) && directCall(node.expression, 'writeFileSync')) {
   const [destination, serialization] = node.expression.arguments;
   const resultPath = directCall(destination, 'resolve') && destination.arguments.length === 2
    && named(destination.arguments[0], 'dir') && literal(destination.arguments[1], 'public/result.json');
   const stringify = serialization && ts.isCallExpression(serialization)
    && ts.isPropertyAccessExpression(serialization.expression)
    && named(serialization.expression.expression, 'JSON') && named(serialization.expression.name, 'stringify');
   const object = stringify && serialization.arguments[0];
   const passed = object && ts.isObjectLiteralExpression(object) && object.properties.some(property =>
    ts.isPropertyAssignment(property) && (named(property.name, 'result') || literal(property.name, 'result'))
    && literal(property.initializer, 'passed'));
   if (resultPath && passed) candidates.push(node);
  }
  ts.forEachChild(node, visit);
 }
 visit(parsed);
 assert.equal(candidates.length, 1, `${filename}: expected exactly one passed publication statement`);
 return source.slice(candidates[0].getStart(parsed), candidates[0].end);
}

function withReports(api, browser, run) {
 const dir = mkdtempSync(resolve(tmpdir(), 'weaveos-publication-'));
 try {
  mkdirSync(resolve(dir, 'public'));
  writeFileSync(resolve(dir, 'api.tap'), `TAP version 13\n1..${api}\n${Array.from({length: api}, (_, index) => `ok ${index + 1} - fixture ${index + 1}`).join('\n')}\n# tests ${api}\n# pass ${api}\n# fail 0\n# cancelled 0\n# skipped 0\n# todo 0\n`);
  writeFileSync(resolve(dir, 'browser.json'), JSON.stringify({stats: {expected: browser, unexpected: 0, flaky: 0, skipped: 0}}));
  writeFileSync(resolve(dir, 'components.json'), JSON.stringify({stats: {expected: 3}}));
  const output = resolve(dir, 'public/result.json');
  const sentinel = JSON.stringify({result: 'running', marker: 'publication-has-not-run'});
  writeFileSync(output, sentinel);
  const inputs = new Set(['api.tap', 'browser.json', 'components.json'].map(file => resolve(dir, file)));
  let writes = 0;
  const context = {
   dir, project: 'publication-fixture', started: 1000, start: 1000, Date: {now: () => 2000},
   artifacts: {commit: 'fixture-current', bff: {manifestDigest: 'fixture-bff', imageID: 'fixture-bff-id'}, web: {manifestDigest: 'fixture-web', imageID: 'fixture-web-id'}},
   previous: {commit: 'fixture-previous'}, resolve, JSON, countAPIReport, countBrowserReport,
   readFileSync(path, encoding) {
    assert.ok(inputs.has(path), `unexpected publication input: ${path}`);
    assert.equal(encoding, 'utf8');
    return readFileSync(path, encoding);
   },
   writeFileSync(path, bytes) {
    assert.equal(path, output);
    writeFileSync(path, bytes);
    writes++;
   },
  };
  run({dir, output, sentinel, writes: () => writes,
   execute: (statement, filename) => new Script(statement, {filename}).runInNewContext(context,
    {timeout: 1000, contextCodeGeneration: {strings: false, wasm: false}})});
 } finally {
  rmSync(dir, {recursive: true, force: true});
 }
}

test('publication selector requires one real passed statement, not source-text decoys', () => {
 const statement = "writeFileSync(resolve(dir,'public/result.json'),JSON.stringify({result:'passed',api:42,browser:43}));";
 assert.equal(publicationStatement(`// ${statement}\nconst decoy = ${JSON.stringify(statement)};\nfunction publish(){${statement}}`, 'selector.js'), statement);
 for (const source of ['', `// ${statement}`, `const decoy = ${JSON.stringify(statement)};`, `${statement}\n${statement}`]) {
  assert.throws(() => publicationStatement(source, 'selector.js'), {code: 'ERR_ASSERTION', message: /expected exactly one passed publication statement/});
 }
 assert.throws(() => publicationStatement('function {', 'selector.js'), {code: 'ERR_ASSERTION', message: /invalid JavaScript/});
});

for (const path of runners) {
 test('actual passed publication writes independent report counts: ' + path, () => {
  const statement = publicationStatement(readFileSync(path, 'utf8'), path);
  for (const [api, browser] of [[2, 7], [9, 13]]) withReports(api, browser, fixture => {
   fixture.execute(statement, path);
   assert.equal(fixture.writes(), 1, 'exactly one actual result write');
   const result = JSON.parse(readFileSync(fixture.output, 'utf8'));
   assert.equal(result.result, 'passed');
   assert.deepEqual({api: result.api, browser: result.browser}, {api, browser}, 'published counts must equal the independent report fixture');
  });
 });
 test('bad reports cannot reach the passed publication write: ' + path, () => {
  const statement = publicationStatement(readFileSync(path, 'utf8'), path);
  const cases = [
   {name: 'contradictory API footer', prepare: dir => {
    const file = resolve(dir, 'api.tap');
    writeFileSync(file, readFileSync(file, 'utf8').replace('# pass 2', '# pass 1'));
   }, matches: error => error instanceof Error && error.message === 'API report is not fully passed'},
   {name: 'failed browser report', prepare: dir => writeFileSync(resolve(dir, 'browser.json'), JSON.stringify({stats: {expected: 7, unexpected: 1, flaky: 0, skipped: 0}})),
    matches: error => error instanceof Error && error.message === 'Browser report is not fully passed'},
   {name: 'malformed browser JSON', prepare: dir => writeFileSync(resolve(dir, 'browser.json'), '{'),
    matches: error => error instanceof SyntaxError},
   {name: 'missing API report', prepare: dir => unlinkSync(resolve(dir, 'api.tap')),
    matches: (error, dir) => error.code === 'ENOENT' && error.path === resolve(dir, 'api.tap')},
  ];
  for (const item of cases) withReports(2, 7, fixture => {
   item.prepare(fixture.dir);
   assert.throws(() => fixture.execute(statement, path), error => item.matches(error, fixture.dir), item.name);
   assert.equal(fixture.writes(), 0, `${item.name}: no passed write`);
   assert.equal(readFileSync(fixture.output, 'utf8'), fixture.sentinel, `${item.name}: running sentinel unchanged`);
  });
 });
}
