#!/usr/bin/env python3
"""Require branch CI for the release commit without rerunning its tests/builds."""

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import time


def load_runs(repository, sha):
    result = subprocess.run([
        'gh', 'api', '--method', 'GET', '--paginate', '--slurp',
        f'repos/{repository}/actions/workflows/ci.yml/runs',
        '-f', f'head_sha={sha}', '-f', 'event=push', '-f', 'per_page=100',
    ], check=True, capture_output=True, text=True, timeout=60)
    return [run for page in json.loads(result.stdout) for run in page['workflow_runs']]


def latest_branch_run(runs, repository, sha):
    candidates = [run for run in runs if (
        run['head_sha'] == sha
        and run['event'] == 'push'
        and run['head_branch'] in ('develop', 'main')
        and (run.get('head_repository') or {}).get('full_name', '').lower()
        == repository.lower()
    )]
    # A newer failed/running check must not be hidden by an older success.
    return max(candidates, key=lambda run: run['id'], default=None)


def wait_for_ci(repository, sha, timeout=2400):
    started = time.monotonic()
    deadline = started + timeout
    # Branch and tag pushes can arrive together, before CI appears in the API.
    discovery_deadline = min(deadline, started + 120)
    while True:
        run = latest_branch_run(load_runs(repository, sha), repository, sha)
        if run and run['status'] == 'completed':
            if run['conclusion'] != 'success':
                raise RuntimeError(
                    f"CI for {sha} finished with {run['conclusion']}: {run['html_url']}. "
                    'Fix or rerun branch CI before retrying the release.'
                )
            return run

        now = time.monotonic()
        if run is None:
            if now >= discovery_deadline:
                raise RuntimeError(
                    f'No branch CI found for {sha}. Push this commit to develop or main '
                    'and let CI pass before retrying the release.'
                )
            remaining = discovery_deadline - now
            print(f'Waiting for branch CI to appear for {sha}...', flush=True)
        else:
            if now >= deadline:
                raise RuntimeError(f"Timed out waiting for CI: {run['html_url']}")
            remaining = deadline - now
            print(f"Waiting for CI ({run['status']}): {run['html_url']}", flush=True)
        time.sleep(min(20, remaining))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sha', required=True)
    parser.add_argument('--repository', default=os.environ.get('GITHUB_REPOSITORY'))
    args = parser.parse_args()
    if not args.repository:
        parser.error('--repository or GITHUB_REPOSITORY is required')

    try:
        run = wait_for_ci(args.repository, args.sha)
    except (RuntimeError, subprocess.SubprocessError) as error:
        print(f'::error::{error}', file=sys.stderr)
        return 1

    print(f"Reusing successful CI for {args.sha}: {run['html_url']}")
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with Path(os.environ['GITHUB_STEP_SUMMARY']).open('a') as summary:
            summary.write(
                f"### Verified commit\n\nReusing [branch CI]({run['html_url']}) "
                f"for `{args.sha}`. Tests and CI images are not rebuilt in Release.\n"
            )
    return 0


if __name__ == '__main__':
    sys.exit(main())
