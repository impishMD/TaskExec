#!/usr/bin/env bash
set -euo pipefail
format=${1:?deb or rpm required}
artifacts=$(cd "${2:-dist}" && pwd)
arch=$(docker info --format '{{.Architecture}}')
case "$arch" in aarch64|arm64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) exit 1 ;; esac
package=$(find "$artifacts" -maxdepth 1 -name "*_linux_${arch}.$format" -print -quit)
test -n "$package"
repo=$(cd "$(dirname "$0")/.." && pwd)
case "$format" in
  deb) base=ubuntu:24.04 ;;
  rpm) base=rockylinux:9 ;;
  *) exit 1 ;;
esac
base="${TASKEXEC_TEST_REGISTRY:-docker.io}/library/$base"
docker run --rm -v "$artifacts:/packages:ro" -v "$repo/tools:/tests:ro" \
  -e PACKAGE="/packages/$(basename "$package")" -e FORMAT="$format" -e EXPECTED_VERSION="v$(cat "$repo/VERSION")" "$base" bash -euc '
  if [[ "$FORMAT" == deb ]]; then
    apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl python3 openssl "$PACKAGE"
  else
    dnf install -y -q curl-minimal python3 openssl "$PACKAGE"
  fi
  test -f /usr/lib/systemd/system/taskexec.service
  test -f /usr/share/licenses/taskexec/NOTICE
  id taskexec
  test "$(stat -c %U /etc/taskexec)" = taskexec
  runuser -u taskexec -- bash /tests/test-binary.sh /usr/bin/taskexec "$EXPECTED_VERSION"
  printf retained > /etc/taskexec/retention-check
  printf retained > /var/lib/taskexec/retention-check
  if [[ "$FORMAT" == deb ]]; then dpkg --remove taskexec; else rpm -e taskexec; fi
  test "$(cat /etc/taskexec/retention-check)" = retained
  test "$(cat /var/lib/taskexec/retention-check)" = retained
'
