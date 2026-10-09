# V030-055 · Hosted disk preparation oracle

## Independent requirement and scope

The V010-008 historical task and RED/GREEN record restrict CI disk preparation to GitHub-hosted Linux and the unused Android SDK. V030-048 requires preservation of cleanup, evidence and permission boundaries. See this task's Notion PRD and ADR in `docs/tasks/V030-055.md`.

This slice adds real Bash control-flow observation to the existing Python preflight entry. It does not change the workflow, implementation, dependencies, old eight Python contracts, or six runtime configuration tests. Their whole-workflow Docker prune prohibition remains. This does not close all ORACLE-004/020/028 findings.

## Actual sequence

1. Fixed baseline `0ab6986caf356ea8bfa3d61ec2c39ca13f30df88`: old full configuration suite 6 PASS, then four isolated variants each 1 PASS under the old disk text assertion, then original suite 6 PASS again. No variant shell ran in this before phase.
2. Root's new test was transferred, checked and applied. An unexpanded message placeholder and an extra EOF context line in the first patch were corrected before application; they are not behavioral RED. The actual v2 patch and received first patch are retained.
3. New baseline 11 PASS (original 8 plus 3 new methods). The new class covers 24 ineligible environment/OS combinations, the single authorized cleanup request and cleanup failure propagation.
4. Same isolated variants: late guards produce 24 failing subtests; extra target causes 2 failures; comments-only causes 26 failures; conditional skip causes 3 failures. Each exits 1 with the intended AssertionError, not import, syntax or environment ERROR. Main test count and subtest failure count are distinct. Restored old 6 and new 11 pass.
5. Frozen installation, Python 11, Node governance/foundation/contracts 390, runtime config 6 and typecheck pass. Root supplied an invalid `pnpm lint` command; its exit 254 is retained. The existing actual `pnpm exec redocly lint contracts/openapi/openapi.json` passes with 12 pre-existing warnings. Build, check-tasks, verify-repo and diff-check pass; existing chunk warning remains. Two document EOF whitespace warnings were corrected without test changes.

## What the behavior test proves

The original YAML run body is passed to the real Bash executable with GitHub's documented explicit-bash flags. External sudo/rm/df/docker commands are argument-recording stubs; the sudo stub never invokes rm. The child has a minimal synthetic environment and a PATH containing only these stubs. No SDK, Docker resource or real machine data is deleted.

This is a test of reviewed repository code, not an arbitrary hostile-shell sandbox. PATH stubs do not intercept absolute executables or shell built-in file writes; those are not present in the reviewed input. Synthetic RUNNER_OS values do not mean execution on actual Windows/macOS. It does not prove real disk capacity, system utility behavior or whole-workflow Actions reachability. Real hosted positive execution remains separately checked in the final exact-candidate CI.

## Recoverable records

`execution-records.json` stores original UTF-8 text, per-file size and SHA256 for the measured files, scripts, patches, baseline sources and final candidate test. `manifest.json` lists all entries and the bundle digest. Each record is recoverable as `text.encode('utf-8')`; recheck length and SHA256 before replay. Replay is not original execution.

The frozen bundle includes the first invalid lint output and intended mutant failures. Subsequent archive-validation and exact-HEAD scan logs are separate checkpoints and must not be misrepresented as already inside this earlier snapshot. Final CI/merge facts are archived in Notion and the PR, without rewriting failure history or manufacturing an after-the-fact first-pass result. Main approval, release risk acceptance and deployment are not granted by this evidence.
