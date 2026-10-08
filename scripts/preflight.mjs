import {spawnSync} from 'node:child_process';
import {readdirSync} from 'node:fs';
import {resolve} from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

function executeStep({command, args, cwd, env}) {
  // Expand the existing flat *.test.mjs selections without shell evaluation.
  const directories = new Set(['tests/governance', 'tests/foundation', 'contracts']);
  const expanded = args.flatMap(arg => {
    if (!directories.has(arg)) return [arg];
    const files = readdirSync(resolve(cwd, arg)).filter(name => name.endsWith('.test.mjs')).sort();
    if (files.length === 0) throw new Error('Required test directory is empty');
    return files.map(name => `${arg}/${name}`);
  });
  return spawnSync(command, expanded, {cwd, env, encoding: 'utf8', timeout: 300000, maxBuffer: 8 * 1024 * 1024});
}

export function runPreflight({root = fileURLToPath(new URL('..', import.meta.url)), env = process.env, execute = executeStep} = {}) {
  const steps = [
    {id: 'policy-tests', command: 'node', args: ['--test', 'tests/governance', 'tests/foundation']},
    {id: 'repository', command: 'node', args: ['scripts/verify-repo.mjs']},
    {id: 'tasks', command: 'node', args: ['scripts/check-tasks.mjs']},
    {id: 'format', command: 'gofmt', args: ['-l', '.'], cwd: resolve(root, 'services/bff')},
    {id: 'contract-tests', command: 'node', args: ['--test', 'contracts']},
    {id: 'contract-lint', command: 'pnpm', args: ['exec', 'redocly', 'lint', 'contracts/openapi/openapi.json']},
    {id: 'secrets', command: 'node', args: ['--test', '--test-name-pattern=^tracked source archive passes a redacted Gitleaks secret scan$', 'infra/runtime/security.test.mjs']},
  ];
  for (const step of steps) {
    const failure = `Preflight check failed: ${step.id}`;
    let result;
    try {
      result = execute({...step, cwd: step.cwd ?? root, env});
    } catch {
      throw new Error(failure);
    }
    if (!result || result.error || result.signal || result.status !== 0 ||
        (step.id === 'format' && String(result.stdout ?? '').length !== 0)) {
      throw new Error(failure);
    }
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    runPreflight();
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
