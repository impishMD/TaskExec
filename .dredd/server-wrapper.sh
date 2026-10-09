#!/usr/bin/env bash

export TASKEXEC_MAX_TASKS_PER_TEMPLATE=300
export TASKEXEC_APPS='{"ansible": {}}'
export TASKEXEC_PORT=58427

taskexec=./taskexec
[[ -x "$taskexec" ]] || taskexec=./bin/taskexec

exec "$taskexec" server --config .dredd/config.json
