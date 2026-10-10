# Releasing

**English** | [Русский](../ru/releasing.md)

`VERSION` contains the version without `v`. Tags, release titles and container tags
use `v<major>.<minor>.<patch>`. The default branch is `develop`.

Choose an unused version and tag for every release. Container repositories are
`impishmd/taskexec` and `ghcr.io/impishmd/taskexec`.

## Prepare

1. Update `VERSION`, frontend package versions and deployment examples.
2. Update `CHANGELOG.md` and add `release-notes/en/v<version>.md` and `release-notes/ru/v<version>.md` for GitHub Releases. Keep user guides focused on current behavior.
3. Run `make test`, `make check`, `make build`, runtime smoke tests, and `make release-check`.
4. Commit/push changes and wait for CI and Packages to pass.
5. Tag that verified commit and push the tag:

   ```sh
   release_tag="v$(cat VERSION)"
   git tag -a "$release_tag" -m "$release_tag"
   git push origin "$release_tag"
   ```

Never move an existing release tag. Local `make release-snapshot` builds without publishing.
GoReleaser 2.18.2 defines four Linux/macOS archives (amd64/arm64), four Linux DEB/RPM packages,
a source archive and `checksums.txt`. Packages are not GPG signed. Binary archives include
both documentation languages and license notices.

## Credentials

The GitHub environment **docker_hub** contains **DOCKERHUB_USR** and **DOCKERHUB_TOKEN**.
GHCR uses the workflow `GITHUB_TOKEN` with `packages: write`. Build/test jobs use read access.
Environment protection rules remain effective. Do not store token values in this repository.

## Publication

The Release workflow requires successful CI from a push to `develop` or `main` for the
exact tagged commit. It waits for an ongoing run; missing or failed CI blocks publication.
Tests (including `go test -race`) and temporary CI container builds run in branch CI only.
The check uses the [workflow runs API](https://docs.github.com/en/rest/actions/workflow-runs#list-workflow-runs-for-a-workflow).

Release validates the version and builds/tests release packages before creating a draft.
Branch packages are `-dev` snapshots; CI containers have the embedded version `dev` and
are not release artifacts. Native amd64/arm64 jobs build versioned server, runner, job and
helper images once, push them by digest to Docker Hub and GHCR, and smoke-test those images.
The final job reuses the verified digests without rebuilding: it creates and checks
multi-platform version tags, uploads `container-digests.txt`, updates stable aliases and
publishes the GitHub Release. Prereleases do not update `latest` aliases. Releases are serialized.

Images: `impishmd/taskexec` and `ghcr.io/impishmd/taskexec`. GitHub assets:
[Releases](https://github.com/impishMD/TaskExec/releases).
The pipeline follows [Docker multi-platform guidance](https://docs.docker.com/build/ci/github-actions/multi-platform/)
and [GitHub Container registry authentication](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry).

If publication fails, rerun failed jobs to resume the draft. Published releases are not
overwritten. Publication to two registries is not atomic; retries finish partial publication.
Do not publish an older stable tag after a newer one: that would move the stable aliases back.
Repository and package visibility are managed separately from the workflow.

## Helm chart releases

Application and chart tags are separate: `v1.1.0` publishes the application and
`chart-v1.0.2` publishes chart `1.0.2`. Chart-only changes increment `Chart.yaml`'s `version`;
`appVersion` follows `VERSION`. Helm CI validates Helm 3/4 renders and Kubernetes schemas,
then installs the built server in a disposable kind cluster and tests authentication,
upgrade/restart persistence, URL subpaths and retained-PVC reuse.

Before the first chart publication, enable **Settings → Pages → Source: GitHub Actions**.
Allow `chart-v*` tags in the `github-pages` environment's deployment rules. The workflow
uses `GITHUB_TOKEN` to create GitHub Releases and maintain `gh-pages`; no chart-specific
personal access token is required. Ensure package visibility permits pulling the default
GHCR image, or configure `imagePullSecrets` for users of a private installation.

1. Publish the matching application image first and wait for Release to finish.
2. Set `charts/taskexec/Chart.yaml` versions, validate the chart, and write release notes in
   `release-notes/charts/en/v<chart-version>.md` and `release-notes/charts/ru/v<chart-version>.md`.
3. Commit and push; wait for Helm CI. Tag that commit with `chart-v<chart-version>` and push the tag.
4. Helm Release reruns checks, verifies that the application image exists, packages the chart,
   uploads it to a separate GitHub Release and publishes the repository index through Pages.

Repository URL: `https://impishmd.github.io/TaskExec`. The first release creates `gh-pages`
automatically. Chart releases do not replace the application's GitHub “Latest” release.
Published archives are reused on retry so index digests stay consistent. Resume a failed
chart publication using **Helm Release → Run workflow**, specifying the existing chart tag;
never move an existing tag or reuse its version for different chart content.

Local checks (Python requires `PyYAML==6.0.3`):

```sh
make version-check helm-check
python3 tools/check-chart.py --release-tag chart-v1.0.2
helm package charts/taskexec --destination /tmp/taskexec-charts
```

To exercise installation locally with Docker and kind, use a dedicated kubeconfig:

```sh
export KUBECONFIG=$(mktemp)
kind create cluster --name taskexec-chart --kubeconfig "$KUBECONFIG"
docker build --target server --build-arg VERSION="v$(cat VERSION)" -t taskexec:helm-ci .
kind load docker-image taskexec:helm-ci --name taskexec-chart
bash tools/test-chart-install.sh
kind delete cluster --name taskexec-chart
rm "$KUBECONFIG"
unset KUBECONFIG
```

The install test refuses any current context other than `kind-taskexec-chart`.
