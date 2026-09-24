import { readFileSync } from 'node:fs';
import { releaseErrors } from './release-policy.mjs';
try {
  const matrix = JSON.parse(readFileSync(new URL('../docs/acceptance/v0.1.0.json', import.meta.url), 'utf8'));
  const errors = releaseErrors(matrix);
  if (errors.length) { console.error(errors.join('\n')); process.exitCode = 1; }
  else console.log('Recorded release evidence is structurally complete; CI runs and human sign-off must still be verified.');
} catch (error) { console.error(`[RELEASE] ${error.message}`); process.exitCode = 1; }
