# Original trigger-reservation declaration RED

Exact source0c5daf0c32ca98f9013347358904afddd530a041. Frozen six cases, exit1,6FAIL/0PASS/0SKIP, all actual declaration behavior RED. Real PG fixtures/deployment setup succeeded; stub ReserveTriggeredInTx returns ErrNotReady. Five cases fail at first reservation (lines24,42,56,122,149); concurrent test receives8ErrNotReady (line105), created0/identitiesempty(line116). Later duplicate/replay/closing/rollback/stale-version assertions remain unreached at this declaration stage. No compiler/environment/fixture failure; empty stderr/no race report.

Original command `go test -race -p 1 -count=1 -json -run '^TestRootTriggerReservation' ./internal/apprecordservice`; same private hot/archivePG18.6/Redis8.2.10, sequential after V046 completed, migrations1..21 applied. Original JSONL/run.json/stderr/exit and immutable test/stub snapshots byte-preserved; before/after hashes unchanged. Historical original run remains original, not a replay. Root owns independent assertion review. TDD:N/A for evidence-only preservation.

| Byte-preserved artifact | Bytes | SHA256 |
| --- | --- | --- |
| run.json | 5377 | 42f44578867f88be1cb80ab7fa1b1b1fdc880ad05df21ad4d80e1a94dceb1d9e |
| test.exit | 2 | 4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865 |
| test.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| test.stdout.jsonl | 11260 | f229b41f7a3732b41f390ea2eb1928c4162a7fde340b9b467f6de855c5822149 |
| trigger_reservation.go | 304 | 801540856120a8c1955393d411de226befe6b027a49219c92a71fa60d193b586 |
| root_workflow_trigger_reservation_test.go | 5480 | 1fa4277e0370f70d2258a5c1033411cd9163fcca4223a96ee3b3058268fc9c0d |

Original test snapshot has an original terminal blank line; it is stored losslessly as root_workflow_trigger_reservation_test.go.gz. Original/private/current-test SHA256 remains `1fa4277e0370f70d2258a5c1033411cd9163fcca4223a96ee3b3058268fc9c0d`; decompressed bytes are identical. Compressed artifact SHA256 `679f187912feb757cdc2a025cccaecfd4c60f52f9de6c8e266f63ff44bc93127`. Captured run.json hashes remain the original-byte hashes and are not rewritten.
