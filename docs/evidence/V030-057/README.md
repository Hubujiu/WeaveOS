# V030-057 evidence

## Contract and scope

Original ORACLE002 requires publication counts to equal independent report inputs, not merely the presence of parser calls. See the task's prior Notion PRD/ADR. Only `contracts/result-publication.test.mjs` is added; the original six governance tests, production parsers, both runners, workflows and dependencies remain byte-identical to develop `a29cd5502c5f822bd46d15e5a69a4447cede52e3`.

TypeScript 5.9.3's standard AST selects exactly one real passed publication expression. Standard Node `vm.Script` executes its unchanged source slice, using real parsers and real temporary files. API/browser expectations are independent pairs 2/7 and 9/13. Invalid reports must produce the specified parser/JSON/filesystem error, zero writes and unchanged running sentinel. This is an extracted-statement test, not proof of whole-runner reachability, import bindings, report production or catch/finally. `vm` is not a security sandbox.

## Actual execution

- Fixed local Node 24.14.0 / pnpm 10.28.2; official downloads checked against vendor checksums. Existing TypeScript lock remains unchanged. Saved cloud task service was unavailable; the user explicitly requested execution on the assistant's computer.
- Before: baseline six old tests PASS; each isolated runner discard/swap variant still six PASS; restored six PASS.
- After: five new tests PASS. Each of the same four variants has exactly four PASS / one FAIL, at the independent count assertion after exactly one real result write. Discard writes 42/43; swap writes 7/2; expected 2/7. No selector, syntax, import or environment failure is counted as behavior RED. Restored five PASS, zero skipped.
- Combined governance/foundation/contracts: 395 PASS / zero FAIL / zero skipped. Typecheck, actual Redocly lint, build, task, structure and diff checks exit zero. Existing OpenAPI/build warnings are retained in the raw logs.
- Initial installation launch failed before pnpm because its log directory was created relative to the worktree; it was corrected to an absolute path. This is preserved as an environment mistake, not test RED.

## Recoverable records

`execution-records.json` contains 127 UTF-8 records: measured commands, output/errors/exit codes, isolated original and mutant sources, summaries, Root measurement/test/archive scripts and tool manifest. Bundle size 520203 bytes; SHA256 `ca3e7426710ed6b93a7a2c12a708ddb16b0343c4bd481c6415f97c49cbfde90f`. `manifest.json` gives each record's byte count and SHA256. Recover a record's exact bytes by UTF-8 encoding its `text` and verifying both fields. Dependency symlinks, caches, binaries and unrelated private files are excluded.

The archive script snapshots the execution folder before later final checks. Exact candidate scans, Git publication and complete CI are subsequent stages, not fabricated as part of this snapshot. The assistant machine has no Docker executable; any local same-version native Gitleaks check must be identified separately from the unchanged fixed-image security test in full GitHub CI.

## Entry points and pending acceptance

The standalone governance job has no installed TypeScript dependency. The new test is therefore in top-level `contracts/`, included by existing installed-dependency preflight, browser and product checks. No missing-dependency skip or workflow modification is used. The product container's Node version and compatibility must be established by its actual execution, not inferred from the host `.node-version`.

At this snapshot, exact final HEAD scanning and complete CI are pending. Development integration, main approval and deployment remain separate facts.
