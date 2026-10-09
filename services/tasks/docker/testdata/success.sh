#!/bin/sh
set -eu
test "$CONTAINER_TEST_SECRET" = "fixture-only-secret"
test -z "${TASKEXEC_RUNNER_TOKEN:-}"
printf 'CONTAINER_SUCCESS\n'
