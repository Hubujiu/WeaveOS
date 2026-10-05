# Read-only security diagnosis

Frozen1821f8c, no repository changes. Pending Root/user disposition; no dependency bump/downgrade, gate exception, whitelist or test edits. The module scan is run from services/bff/cmd/bff; the source symbol scan covers ./... from services/bff. Both use locked Go1.27.1 and govulncheck1.8.0. Raw JSON streams are multiple top-level objects, not JSON arrays. Symbol JSON contains module/package findings but no reachable symbol finding; do not equate exit0 with absence of module findings.

Commands:
```sh
go -C services/bff/cmd/bff run golang.org/x/vuln/cmd/govulncheck@v1.8.0 -json -scan module
go -C services/bff run golang.org/x/vuln/cmd/govulncheck@v1.8.0 -json -scan symbol ./...
go -C services/bff list -m -json all
go -C services/bff mod graph
go -C services/bff list -deps ./cmd/bff
go -C services/bff mod verify
```

The application directly selectsgrpc1.84.0; x/vuln is not a selected application module. grpc's transport package is imported, xDS server is absent. Source downloaded from upstream tag is byte-identical to module cache; sumdb matches module/go.mod sums and go mod verify succeeds. The release tag includes backportd5a41119 (ahead3/behind0). Official patched-version declaration conflicts with currentOSV1.84.0-dev-to-1.85.0-dev interval. This packet proves the conflict; it does not grant an exception or deployment approval.

Failed product CI logs and job-step metadata retained; primary6CI successful, productmigration/HTTPS successful, securitygate failsGO-2026-6443 and subsequent runtime/deployment packaging unexecuted. Raw source is upstream Apache2.0 code with original header. SHA256SUMS covers every packet file except itself.
