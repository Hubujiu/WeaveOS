# Formal process acceptance at 95025dca

Root personally inspected actual test output and verified artifact SHA256 `2d55c2e012f90505f59d38302c8c9a42cb19f932128a21db35be1e52af3d9220` (artifact11425113098, CI37490749804/job112362528089). PR merge6b9fbecd has parents develop8fc8f295 and head95025dca; its tree7141aad746c8406b1da65457b652f99085397824 equals the exact task source tree. Three scenarios and agree/reject variants all pass, no skipped cases; independent exact gate was rerun locally against downloaded JSONL and passes.

The actual BFF and WorkflowEngineMain binary paths run in independent OS child processes. Java classpath is runtime-only. Real HTTPS, Session/CSRF and service-authenticated loopback gRPC reach native Flowable and separate migrated application/engine PostgreSQL. Internal accepted-start fixture is consumed only by formal BFF workers; this does not cover public new-record trigger configuration or end-to-end trigger acceptance.

Scenarios cover definition save/publication/enable, agree/reject and original receipt, lost genuine committed response followed by BFF kill/restart with receipt lookup (one execution effect), and Java kill/restart with unavailable new operation rejected without consuming its ID/token then successful retry. Isolation report confirms internal network and zero published ports; runner inspection also checks no Docker socket. No deployment or real credential/network configuration.

Observed CI samples: fixture setup plus program startup 2712–2970 ms; post-acceptance convergence 271–378 ms; BFF child peak RSS 30492–33868 KiB; Java child peak RSS244844–264180 KiB. These are low-volume synthetic CI samples, not end-to-end user latency, steady-state RSS, throughput or a production SLA. Java maximum heap384MiB, fixture runtime pool2/RPC threads2/queue8; prior pool tests independently cover resource bounds. Full product and browser regressions remain separate pending checks.

Archive is base64 ZIP with original digest above. Full job log is losslessly compressed, digest in compressed-evidence.txt. No tests or runtime implementation changed after this successful execution.
