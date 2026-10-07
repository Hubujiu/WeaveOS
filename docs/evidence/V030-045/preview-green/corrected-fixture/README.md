# Corrected historical fixture: actual preview and service GREEN

Exact source014e3529acd9fa45dbcb225fe0ecedb02cd7dbc3, parentedd030e candidate; only Root's historical fixture definitions changed, no production change. Frozen `go test -race -p 1 -count=1 -json -run '^TestRoot(LifecyclePreview|WorkflowLifecycle)' ./internal/apprecordservice`: exit0,7previewPASS+6pure-qualificationPASS,0FAIL/0SKIP. Separate entire default service raceexit0:179top-levelPASS/93subcasePASS, includes7preview+6qualification+166other-top/93other-sub. No filter/skip in full service. All stderr empty, no race report. Original3second single-pool bound unchanged.

Original JSONL/run.json/stderr/exits and exact test/production snapshots byte-preserved; source/test/dependency hashes unchanged during execution. First failed run remains byte-identical and independently indexed next door. Exact argv/environment/times/cases/source hashes in run.json. Same authorized private hot/archivePG18.6/Redis8.2.10 reused sequentially with no reset/overlap. These cover only frozen preview/qualification and service regression, not public withdraw/return CAS/command/HTTP/Flowable/fullCI/final release. Root owns remaining test/implementation/review. TDD:N/A for evidence-only preservation. No executor debug/test/production modification, PR, merge or deployment.

| Byte-preserved artifact | Bytes | SHA256 |
| --- | --- | --- |
| race-apprecordservice.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| race-apprecordservice.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| race-apprecordservice.stdout.jsonl | 366229 | b8a9ec2d073bc23692936c85a290f8a4257610fbe1508e0ccccf5b9c2257a396 |
| run.json | 52676 | f2083bfec290eff51d484731d25f00ca863adb1f4ac3c9e3b0b08b688ac2b6b5 |
| preview-and-policy.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| preview-and-policy.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| preview-and-policy.stdout.jsonl | 13927 | 5cf3cdd8f939d056ca616f9fa1f0f64b772e734cd6a9880024a3f9433f241807 |
| workflow_lifecycle_policy.go | 1420 | dcf001030a2abae5f3a229f147556e101c9ae55d311c07e63371bdc8315f05c4 |
| workflow_lifecycle_preview.go | 12144 | 948a164015b19a0807b933de77aa4c411bf72ce3d03ed1bb7f91e27a3effa390 |
| root_workflow_lifecycle_policy_test.go | 4291 | 971d9acdfc0febbd654b1e8cc2a5a0c9228f018db1c58cb1af8bc7394176653b |
| root_workflow_lifecycle_preview_test.go | 5204 | f5b599a5f6d8a7533bc64484eacb75b08c0f044b4b0309d698da50dc850a95e0 |
