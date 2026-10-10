"""Release must use a successful CI run for its exact commit, not a nearby one."""

import importlib.util
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    'release_ci', Path(__file__).with_name('check-release-ci.py'))
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)

REPOSITORY = 'impishMD/TaskExec'
SHA = 'a' * 40


def run(**overrides):
    return {
        'id': 100,
        'head_sha': SHA,
        'head_repository': {'full_name': REPOSITORY},
        'head_branch': 'develop',
        'event': 'push',
        'status': 'completed',
        'conclusion': 'success',
        'html_url': 'https://github.com/impishMD/TaskExec/actions/runs/100',
        **overrides,
    }


class ReleaseCITest(unittest.TestCase):
    def test_reuses_success_without_waiting_or_starting_jobs(self):
        with patch.object(ci, 'load_runs', return_value=[run()]), patch.object(ci.time, 'sleep') as sleep:
            self.assertEqual(ci.wait_for_ci(REPOSITORY, SHA)['id'], 100)
            sleep.assert_not_called()

    def test_rejects_other_commits_events_branches_and_repositories(self):
        for candidate in [
            run(head_sha='b' * 40), run(event='pull_request'),
            run(event='workflow_dispatch'), run(head_branch='feature'),
            run(head_branch='v1.0.7'), run(head_repository={'full_name': 'fork/TaskExec'}),
            run(head_repository=None),
        ]:
            with self.subTest(candidate=candidate):
                self.assertIsNone(ci.latest_branch_run([candidate], REPOSITORY, SHA))

    def test_main_and_case_insensitive_repository_are_accepted(self):
        self.assertIsNotNone(ci.latest_branch_run([run(head_branch='main')], REPOSITORY.lower(), SHA))

    def test_failed_or_skipped_run_blocks_release_even_with_older_success(self):
        for conclusion in ['failure', 'cancelled', 'timed_out', 'skipped', 'neutral']:
            with self.subTest(conclusion=conclusion):
                runs = [run(id=101, conclusion=conclusion), run()]
                with patch.object(ci, 'load_runs', return_value=runs):
                    with self.assertRaisesRegex(RuntimeError, conclusion):
                        ci.wait_for_ci(REPOSITORY, SHA)

    def test_waits_for_newer_run_instead_of_using_older_success(self):
        pending = run(id=101, status='in_progress', conclusion=None)
        with patch.object(ci, 'load_runs', side_effect=[
            [run(), pending], [run(id=101)],
        ]), patch.object(ci.time, 'sleep') as sleep, patch('builtins.print'):
            self.assertEqual(ci.wait_for_ci(REPOSITORY, SHA)['id'], 101)
            sleep.assert_called_once()

    def test_allows_ci_to_appear_after_simultaneous_branch_and_tag_push(self):
        with patch.object(ci, 'load_runs', side_effect=[[], [run()]]), \
                patch.object(ci.time, 'sleep') as sleep, patch('builtins.print'):
            self.assertEqual(ci.wait_for_ci(REPOSITORY, SHA)['id'], 100)
            sleep.assert_called_once()

    def test_missing_ci_and_pending_ci_time_out(self):
        for runs, message in [
            ([], 'No branch CI'),
            ([run(status='queued', conclusion=None)], 'Timed out'),
        ]:
            with self.subTest(message=message), patch.object(ci, 'load_runs', return_value=runs), \
                    patch.object(ci.time, 'monotonic', side_effect=[0, 5]):
                with self.assertRaisesRegex(RuntimeError, message):
                    ci.wait_for_ci(REPOSITORY, SHA, timeout=5)

    def test_api_query_reads_all_pages_for_ci_workflow_and_exact_sha(self):
        result = subprocess.CompletedProcess([], 0, stdout=json.dumps([
            {'workflow_runs': [run()]}, {'workflow_runs': [run(id=101)]},
        ]))
        with patch.object(ci.subprocess, 'run', return_value=result) as api:
            self.assertEqual([item['id'] for item in ci.load_runs(REPOSITORY, SHA)], [100, 101])
            command = api.call_args.args[0]
            self.assertIn('--paginate', command)
            self.assertIn(f'head_sha={SHA}', command)
            self.assertIn('event=push', command)
            self.assertIn(f'repos/{REPOSITORY}/actions/workflows/ci.yml/runs', command)
            self.assertEqual(command[command.index('--method') + 1], 'GET')

    def test_api_failure_cannot_approve_release(self):
        with patch.object(ci.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, 'gh')):
            with self.assertRaises(subprocess.CalledProcessError):
                ci.wait_for_ci(REPOSITORY, SHA)


if __name__ == '__main__':
    unittest.main()
