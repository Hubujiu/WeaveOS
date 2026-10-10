# Root integration review, 2026-10-10 04:01 UTC

Actual reviewed dependency d770e693 includes PR74 develop22ddd50b. V064 PR75 is still running its exact-head CI, not merged. Normal merge f2cea906 preserves the original RED/GREEN commits. Only the task index conflicted; both task entries retained.

Against d770e693 the executable delta remains exactly five files: migration28, append-only compatibility registration, publication history contract, and two independent test files. No production Go change. Reviewed invoker fixed search_path triggers, strict READ COMMITTED admission, app/flow transaction advisory serialization, original immutable history fields, app ownership foreign keys, and undrained work refusal. No role expansion or public deletion API. Old migration and role files remain unchanged. Prior actual concurrent lock, populated migration, refusal, backup/restore and HTTP replay evidence remains applicable to byte-identical task code.

Fresh actual PostgreSQL18.6 and Redis8.2.10 ten-package Go1.27.2 race run ended03:57:59:735 top-level and560 subtests passed, zero failures, three existing opt-in capacity probes skipped. Raw JSON was scanned before lossless gzip; raw/gzip SHA256 retained. Node538 passed, no skipped/cancelled tests. OpenAPI zero errors and13 existing warnings with telemetry explicitly off. go vet/build and task structural validation exit0. This is local integration evidence, not final CI/develop acceptance.

Remaining: integrate actual V064 squash develop, exact final PR CI, recheck head and target, then authorized squash into develop. main and deployment remain outside this task.

Exact c5bc16da Git source archive scanned with Gitleaks8.30.1: exit0, zero leaks.
