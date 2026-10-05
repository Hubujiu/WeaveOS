#!/usr/bin/env bash
set -euo pipefail
rpc_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
rpc_root=$(cd "$rpc_dir/../.." && pwd)
rpc_go=${WEAVEOS_RPC_GO:-go}
test "$($rpc_go version | cut -d ' ' -f 3)" = go1.27.1
export GOTOOLCHAIN=local
mkdir -p "$rpc_dir/.work/bin"
GOBIN="$rpc_dir/.work/bin" "$rpc_go" install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
GOBIN="$rpc_dir/.work/bin" "$rpc_go" install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
"$rpc_dir/.work/bin/protoc-gen-go" --version
"$rpc_dir/.work/bin/protoc-gen-go-grpc" --version
JAVA_HOME="$rpc_dir/.work/toolchain/jdk" "$rpc_dir/.work/toolchain/maven/bin/mvn" \
  -B -ntp -o -s "$rpc_dir/maven-settings.xml" -f "$rpc_dir/pom.xml" -Dmaven.repo.local="$rpc_dir/.work/m2" \
  org.xolstice.maven.plugins:protobuf-maven-plugin:0.6.1:compile \
  org.xolstice.maven.plugins:protobuf-maven-plugin:0.6.1:compile-custom
rpc_protoc="$rpc_dir/.work/m2/com/google/protobuf/protoc/3.25.9/protoc-3.25.9-linux-x86_64.exe"
chmod +x "$rpc_protoc"
"$rpc_protoc" --version
PATH="$rpc_dir/.work/bin:$PATH" "$rpc_protoc" -I "$rpc_root/contracts/proto" \
  --go_out="$rpc_root/services/bff" --go_opt=module=github.com/Hubujiu/WeaveOS/services/bff \
  --go-grpc_out="$rpc_root/services/bff" --go-grpc_opt=module=github.com/Hubujiu/WeaveOS/services/bff \
  "$rpc_root/contracts/proto/weaveos/workflow/v1/deployment.proto" \
  "$rpc_root/contracts/proto/weaveos/workflow/v1/execution.proto"
# CI runs this after checkout; generated files must be the exact checked-in artifacts.
git -C "$rpc_root" diff --exit-code -- services/bff/internal/workflowrpc/pb/ services/workflow-engine/src/main/java/org/weaveos/workflow/v1/

# Newly generated, untracked bindings must not escape the reproducibility gate.
if [[ -n "$(git -C "$rpc_root" ls-files --others --exclude-standard -- services/bff/internal/workflowrpc/pb/ services/workflow-engine/src/main/java/org/weaveos/workflow/v1/)" ]]; then
  echo "untracked generated RPC bindings" >&2
  exit 1
fi
