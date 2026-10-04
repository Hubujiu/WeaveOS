#!/bin/sh
set -e
node --check infra/acceptance/run.mjs
node --check infra/runtime/run.mjs
node scripts/check-tasks.mjs
node scripts/verify-repo.mjs
git diff --check -- infra/acceptance/run.mjs infra/runtime/run.mjs
