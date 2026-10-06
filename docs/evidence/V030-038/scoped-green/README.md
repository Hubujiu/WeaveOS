# S0 scoped compiler verification

Root personally ran official Go1.27.1, GOTOOLCHAIN=local and isolated caches.
After integrated declaration RED2FAIL/2PASS at04:30UTC, implemented only one scoped-key caller and extracted the shared existing renderer.
Actual full package `go test -race -json -count=1 ./internal/flowgraph`:27PASS/0FAIL/0SKIP.
`go vet ./internal/flowgraph ./internal/workflowcatalog` and corresponding `go build`:exit0.
Actual existing export tests wrote compiler/return fixtures to a private work directory; `cmp` against four committed Java fixtures all exited0.

Legacy SHA256:
- generated-all:080fb548c65f73471ec8096d2f55fcb39aeeea6447593339465f322ee531134d
- generated-any:9ac8eb04a2d29de94affe7906baecfb26183b42f7c06999196dc7db650f4693d
- return-all:223925a270fb6ff28d026b476ece1e4ffa06c266e978f46eeeccfb82d67c01a6
- return-any:32b08cbe9d5fa072a5c3f31322e4fc6ccb8af1dc31bb38d334cc59207dae420d

Catalog/Java production wiring, native deployment version tests, deletion/audit implementation are not included or claimed complete. Prior original RED evidence and integrated RED are retained.
