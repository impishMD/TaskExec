#!/usr/bin/env bash

export JEH_MAX_TASKS_PER_TEMPLATE=300
export JEH_APPS='{"ansible": {}}'
export JEH_PORT=58427

jeh=./jeh
[[ -x "$jeh" ]] || jeh=./bin/jeh

exec "$jeh" server --config .dredd/config.json
