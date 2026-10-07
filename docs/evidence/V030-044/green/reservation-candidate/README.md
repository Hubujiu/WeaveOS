# Original reservation implementation candidate GREEN

Exact frozen source6db99fac322a4968b1b0e9ac4a818f48337ad043, Root-reviewed/gofmt production candidate. Same unchanged six RootTriggerReservation tests6PASS/0FAIL/0SKIP,exit0. Separate entire apprecordservice default `go test -race -p 1 -count=1 -json ./internal/apprecordservice` exit0:172top-levelPASS/93subcasePASS, includes6newtop-level cases and166oldtop/93oldsub; no skip or full-package name filter. Both stderr empty, no race report. Exact SDK argv/environment/times/cases/source hashes in unchanged run.json and original streams.

Existing real task-private hot/archivePG18.6/Redis8.2.10 reused only after V046 HTTP RED finished; no shared-fixture overlap/reset, no executor implementation/debug/test modification. Test hash matches original RED. Original test and candidate production snapshot included for source-bound independent review. This helper proves only frozen reservation slice; public Create/Edit/HTTP/start intent dispatch/actualFlowable/all-repo/CI/final acceptance remain pending Root's subsequent tests. No PR/merge/deployment. TDD:N/A for evidence-only preservation.

| Byte-preserved artifact | Bytes | SHA256 |
| --- | --- | --- |
| race-apprecordservice.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| race-apprecordservice.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| race-apprecordservice.stdout.jsonl | 359005 | b650d08b39e3b5f11881dbf69f09e1240d563c83b9abca3bfd71493fd1b67219 |
| run.json | 57507 | cca651dc53edaec14cb808095cfd8642b6e7df6e58fbbb41fbe21ddb6edd3fc7 |
| trigger-reservation.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| trigger-reservation.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| trigger-reservation.stdout.jsonl | 6735 | b5cd0c2bb40df6c250c44d78a9c1b90aa342ce25bab461c57d2751bd22ee49d5 |
| trigger_reservation.go | 3628 | b13acc285df921660e5c54ecddbf68ceccbf8b6f69924b18b70d7909ac3305d1 |
| root_workflow_trigger_reservation_test.go | 5480 | 1fa4277e0370f70d2258a5c1033411cd9163fcca4223a96ee3b3058268fc9c0d |

Original test snapshot has an original terminal blank line; it is stored losslessly as root_workflow_trigger_reservation_test.go.gz. Original/private/current-test SHA256 remains `1fa4277e0370f70d2258a5c1033411cd9163fcca4223a96ee3b3058268fc9c0d`; decompressed bytes are identical. Compressed artifact SHA256 `679f187912feb757cdc2a025cccaecfd4c60f52f9de6c8e266f63ff44bc93127`. Captured run.json hashes remain the original-byte hashes and are not rewritten.
