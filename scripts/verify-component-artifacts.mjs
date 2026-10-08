import {execFileSync} from 'node:child_process';
import {mkdirSync, readFileSync, writeFileSync} from 'node:fs';
import {dirname, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {verifyShards} from './verify-shards.mjs';

function requireValid(condition, message) {
  if (!condition) throw new Error(`Invalid component artifacts: ${message}`);
}

const validSha = value => typeof value === 'string' && /^[0-9a-f]{40}$/.test(value);

export function verifyComponentArtifacts({baseline, shards, expectedSha, allowedSkips}) {
  requireValid(validSha(expectedSha), 'expected candidate SHA is required');
  requireValid(baseline && validSha(baseline.sourceSha) && baseline.sourceSha === expectedSha,
    'baseline belongs to a different candidate');
  requireValid(Array.isArray(shards) && shards.length === 4, 'exactly four shards are required');
  const slots = new Set();
  for (const shard of shards) {
    requireValid(shard && validSha(shard.sourceSha) && shard.sourceSha === expectedSha,
      'shard belongs to a different candidate');
    requireValid(shard.shardTotal === 4 && Number.isInteger(shard.shardIndex) &&
      shard.shardIndex >= 1 && shard.shardIndex <= 4 && !slots.has(shard.shardIndex),
    'invalid or duplicate shard slot');
    slots.add(shard.shardIndex);
  }
  const coverage = verifyShards({baseline: baseline.report, reports: shards.map(shard => shard.report), allowedSkips});
  return {sourceSha: expectedSha, scope: 'components',
    stats: {expected: coverage.passed, unexpected: 0, flaky: 0, skipped: coverage.skipped}, coverage};
}

export function loadComponentArtifacts({directory, expectedSha, allowedSkips}) {
  const read = (index, name) => JSON.parse(readFileSync(resolve(directory, `shard-${index}`, name), 'utf8'));
  const discoveries = [];
  const shards = [];
  for (let index = 1; index <= 4; index++) {
    const metadata = read(index, 'metadata.json');
    requireValid(metadata && metadata.shardIndex === index, 'metadata does not match its shard directory');
    discoveries.push({sourceSha: metadata.sourceSha, report: read(index, 'baseline.json')});
    shards.push({...metadata, report: read(index, 'report.json')});
  }
  // Validate the complete actual union against every independent discovery.
  const summaries = discoveries.map(baseline => verifyComponentArtifacts({baseline, shards, expectedSha, allowedSkips}));
  const identities = JSON.stringify(summaries[0].coverage.identities);
  requireValid(summaries.every(summary => JSON.stringify(summary.coverage.identities) === identities),
    'shards discovered different complete identity sets');
  return summaries[0];
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    requireValid(process.argv.length === 4, 'usage: verify-component-artifacts.mjs <shard-directory> <summary-file>');
    const root = fileURLToPath(new URL('..', import.meta.url));
    const expectedSha = execFileSync('git', ['rev-parse', 'HEAD'], {cwd: root, encoding: 'utf8'}).trim();
    const allowedSkips = JSON.parse(readFileSync(resolve(root, 'scripts/component-skips.json'), 'utf8'));
    const summary = loadComponentArtifacts({directory: process.argv[2], expectedSha, allowedSkips});
    const output = resolve(process.argv[3]);
    mkdirSync(dirname(output), {recursive: true});
    writeFileSync(output, JSON.stringify(summary, null, 2) + '\n', {flag: 'wx'});
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
