"""Collect public, global set_stats values for the next workflow nodes."""
import json
import os

from ansible.executor.stats import AggregateStats
from ansible.plugins.callback import CallbackBase


class CallbackModule(CallbackBase):
    CALLBACK_VERSION = 2.0
    CALLBACK_TYPE = "aggregate"
    CALLBACK_NAME = "taskexec_workflow"
    CALLBACK_NEEDS_ENABLED = False

    def __init__(self):
        super().__init__()
        self.public_stats = AggregateStats()

    def v2_runner_on_ok(self, result):
        self.collect(result._result, bool(result._task.no_log))

    def collect(self, result, hidden=False):
        if hidden or result.get("_ansible_no_log"):
            return
        # Loop results are collected once from their final combined result.
        for item in result.get("results", []):
            self.collect(item)
        stats = result.get("ansible_stats", {})
        if stats.get("per_host", False):
            return
        update = (self.public_stats.update_custom_stats if stats.get("aggregate", True)
                  else self.public_stats.set_custom_stats)
        for key, value in stats.get("data", {}).items():
            update(key, value)

    def v2_playbook_on_stats(self, stats):
        output = os.environ.get("TASKEXEC_WORKFLOW_ARTIFACTS_FILE")
        if output:
            with open(output, "w", encoding="utf-8") as stream:
                json.dump(self.public_stats.custom.get("_run", {}), stream)
