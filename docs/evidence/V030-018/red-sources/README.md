# Root RED source snapshots

These are byte-for-byte copies from the initial Root-authored RED commits. The Go files ending in `.stub.go` are compile-only, no-behavior placeholders. They are retained for future squash acceptance; they are not production sources and are not included by their current file names in Go package builds.

| Phase | Source commit | Original path | Snapshot path | SHA256 |
|---|---|---|---|---|
| P1a | `77d39b6aabf1b8f92dedc2a6c879701c55e8b9f8` | `services/bff/internal/flowcommands/protocol.go` | `p1a/protocol.go.stub.go` | `ad4ca7a21df58e5387b58c9a7c20ff4ee107f4a53ac4bce7c3ec45f35649839a` |
| P1a | `77d39b6aabf1b8f92dedc2a6c879701c55e8b9f8` | `services/bff/internal/flowcommands/protocol_test.go` | `p1a/protocol_test.go` | `9e965ab7d91a32fca72ee9c4fc35c882663b7838d6f4b469b0bb780c438c4414` |
| P1b | `2d88fdc9f3d5b7130cdc77bd16112f36c2380a3b` | `services/bff/internal/flowcommands/ledger.go` | `p1b/ledger.go.stub.go` | `310e589549db202f652c3227d310f25966e658c360a4cda38552d333711d70a7` |
| P1b | `2d88fdc9f3d5b7130cdc77bd16112f36c2380a3b` | `services/bff/internal/flowcommands/ledger_test.go` | `p1b/ledger_test.go` | `30be9aa1b6082c589a4bfaf365b28fb734dd0fae0d17e9d25eaf7cc3d173f8c0` |
| P2a | `769ee514b3f248ec2b73ba3cab8223644c2167a2` | `services/bff/internal/apprecordservice/root_command_fence_test.go` | `p2a/root_command_fence_test.go` | `4143e8ed34a9f05e0d908aa719a3f9dc91f287ef28e32f495807ddca50dfe0f5` |
| P2a | `769ee514b3f248ec2b73ba3cab8223644c2167a2` | `docs/evidence/V030-018/p2a/root-red-stubs.sql` | `p2a/root-red-stubs.sql` | `306bbe7f4b31f86c5d1f54051cf95bfa64b1696d51177cbeb02cf8fb2aca9c52` |

The P2a current test file differs only by mechanical gofmt. Formatting the preserved source-commit copy yields exactly the current file bytes; both formatted copies have SHA256 `bc668f93a027c53da7293d3f498039723142a3f81b33ec2122c903ac9095a872`. See `../p2a/base-gofmt-equivalence.sha256`.
