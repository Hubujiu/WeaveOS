#!/usr/bin/env bash
set -euo pipefail
proof_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
proof_image='mirror.gcr.io/library/maven@sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b'
mkdir -p "$proof_dir/.work/toolchain" "$proof_dir/.work/m2"
if [[ ! -x "$proof_dir/.work/toolchain/maven/bin/mvn" ]]; then
  proof_container=$(docker create "$proof_image")
  trap 'docker rm "$proof_container" >/dev/null 2>&1 || true' EXIT
  docker cp "$proof_container:/opt/java/openjdk" "$proof_dir/.work/toolchain/jdk"
  docker cp "$proof_container:/usr/share/maven" "$proof_dir/.work/toolchain/maven"
  docker rm "$proof_container" >/dev/null
  trap - EXIT
fi
# Use the execution environment's existing egress proxy. Never commit private settings.
python3 - "$proof_dir" <<'PY'
import os, pathlib, sys, urllib.parse, xml.etree.ElementTree as ET
base = pathlib.Path(sys.argv[1])
ns = 'http://maven.apache.org/SETTINGS/1.0.0'
ET.register_namespace('', ns)
root = ET.parse(base / 'maven-settings.xml').getroot()
proxy_url = os.environ.get('HTTPS_PROXY') or os.environ.get('HTTP_PROXY')
if proxy_url:
    parsed = urllib.parse.urlsplit(proxy_url)
    proxy = ET.SubElement(ET.SubElement(root, '{'+ns+'}proxies'), '{'+ns+'}proxy')
    values = {'id':'execution-environment-egress', 'active':'true', 'protocol':parsed.scheme,
              'host':parsed.hostname, 'port':str(parsed.port or 80), 'nonProxyHosts':'localhost|127.0.0.1'}
    if parsed.username: values['username'] = urllib.parse.unquote(parsed.username)
    if parsed.password: values['password'] = urllib.parse.unquote(parsed.password)
    for key, value in values.items(): ET.SubElement(proxy, '{'+ns+'}'+key).text = value
path = base / '.work/settings.xml'
ET.ElementTree(root).write(path, encoding='utf-8', xml_declaration=True)
os.chmod(path, 0o600)
PY
trap 'rm -f -- "$proof_dir/.work/settings.xml"' EXIT
proof_java_options=()
if [[ -r /etc/ssl/certs/java/cacerts ]]; then
  proof_java_options+=('-Djavax.net.ssl.trustStore=/etc/ssl/certs/java/cacerts')
fi
JAVA_HOME="$proof_dir/.work/toolchain/jdk" "$proof_dir/.work/toolchain/maven/bin/mvn" \
  -B -ntp -s "$proof_dir/.work/settings.xml" -f "$proof_dir/pom.xml" \
  -Dmaven.repo.local="$proof_dir/.work/m2" "${proof_java_options[@]}" \
  dependency:go-offline
cat > "$proof_dir/.work/provider-pom.xml" <<'XML'
<project xmlns="http://maven.apache.org/POM/4.0.0"><modelVersion>4.0.0</modelVersion><groupId>org.weaveos.proof</groupId><artifactId>test-provider-prefetch</artifactId><version>1</version><dependencies><dependency><groupId>org.apache.maven.surefire</groupId><artifactId>surefire-junit-platform</artifactId><version>3.5.4</version></dependency><dependency><groupId>org.junit.platform</groupId><artifactId>junit-platform-launcher</artifactId><version>1.12.1</version></dependency><dependency><groupId>com.google.protobuf</groupId><artifactId>protoc</artifactId><version>3.25.9</version><type>exe</type><classifier>linux-x86_64</classifier></dependency><dependency><groupId>io.grpc</groupId><artifactId>protoc-gen-grpc-java</artifactId><version>1.84.0</version><type>exe</type><classifier>linux-x86_64</classifier></dependency></dependencies></project>
XML
JAVA_HOME="$proof_dir/.work/toolchain/jdk" "$proof_dir/.work/toolchain/maven/bin/mvn" \
  -B -ntp -s "$proof_dir/.work/settings.xml" -f "$proof_dir/.work/provider-pom.xml" \
  -Dmaven.repo.local="$proof_dir/.work/m2" "${proof_java_options[@]}" \
  org.apache.maven.plugins:maven-dependency-plugin:3.9.0:go-offline
