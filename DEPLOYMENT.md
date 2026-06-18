# Deployment

`go-webext` is a stateless CLI tool distributed as a Go module. There is no
server or infrastructure to deploy. "Deployment" means publishing a new version
that users can install via `go install`.

The package is developed in the private repository
`AdGuardSoftwareLimited/ext-go-webext` and mirrored to the public repository
`AdguardTeam/go-webext`. The Go module path
(`github.com/adguardteam/go-webext`) points to the public mirror so that
`go install` works for external consumers. GitHub Releases are created on the
public mirror.

## Table of Contents

- [CI/CD Workflows](#cicd-workflows)
  - [Build (`build.yml`)](#build-buildyml)
  - [Mirror (`mirror.yml`)](#mirror-mirroryml)
  - [Prepare Release (`prepare-release.yml`)](#prepare-release-prepare-releaseyml)
  - [Publish Release (`publish-release.yml`)](#publish-release-publish-releaseyml)
- [Release Process](#release-process)
  - [Step-by-Step](#step-by-step)
  - [Pipeline Stages](#pipeline-stages)
- [Notifications](#notifications)
- [Environment Variables](#environment-variables)
- [Troubleshooting](#troubleshooting)

## CI/CD Workflows

All workflows live in `.github/workflows/`.

### Build (`build.yml`)

Runs on every pull request and push to `master`.

| Trigger | Purpose |
|---|---|
| `pull_request` | Lint, test, and verify build on PRs |
| `push` to `master` | Lint, test, and verify build on merge |

Jobs (run in parallel):

| Job | Purpose |
|---|---|
| `test` | Run tests via Docker (`docker build --target test-output`) |
| `lint` | Run linters via Docker (`docker build --target lint-output`) |
| `build` | Verify the binary compiles via Docker (`docker build --target build-output`) |

The binary is not published by this workflow — it is only compiled to catch
build breaks early.

### Mirror (`mirror.yml`)

Mirrors code from the private repo to the public mirror
(`AdguardTeam/go-webext`) on every push to `master`, using the shared
`mirror.yml` workflow from `AdGuardSoftwareLimited/actions`.

### Prepare Release (`prepare-release.yml`)

Manually triggered workflow that opens a release pull request.

| Trigger | Purpose |
|---|---|
| `workflow_dispatch` | Manual trigger with tag input (e.g., `v0.5.0`) |

Calls the shared `create-release-pr.yml` workflow from
`AdGuardSoftwareLimited/actions` which:

1. Validates the tag and resolves version metadata via
   `version-metadata.yml`
2. Patches `CHANGELOG.md` — resets `[Unreleased]`, creates a new version
   section with previously-unreleased entries, updates reference links
3. Commits the changelog on a `release-bump/v{version}` branch
4. Opens a PR (attributed to the Octopass app via Octopass token)

**No tags are created by this workflow** — it only opens a PR.

### Publish Release (`publish-release.yml`)

Automatically triggered when a release PR is merged, or manually for
re-runs. Handles tagging, testing, mirroring, release creation, and
notification in a single workflow.

| Trigger | Purpose |
|---|---|
| `pull_request: [closed]` | Auto-fires when a release PR is merged |
| `workflow_dispatch` with ref input | Manual re-run of failed release |

Jobs (sequential, each depends on the previous):

| Job | Runner | Purpose |
|---|---|---|
| `tag` | `team-extensions` (shared workflow) | Parse CHANGELOG, create `v{version}` tag |
| `test` | `team-extensions` | Lint and test via Docker (only on merged release-bump PRs or manual dispatch) |
| `mirror-and-release` | `team-extensions` (shared workflow) | Mirror tag to public repo, create assetless GitHub Release |
| `notify` | `team-extensions` (shared action) | Slack notification |

There is no npm publish step — `go-webext` is a Go module. Consumers install
it via `go install github.com/adguardteam/go-webext@v{version}`.

## Release Process

### Step-by-Step

| # | Who | Action |
|---|-----|--------|
| 1 | Developer | Go to **Actions → Prepare release → Run workflow**, enter tag (e.g., `v0.5.0`) |
| 2 | CI | Opens PR: `release-bump/v0.5.0` → `master` with finalized `CHANGELOG.md` |
| 3 | Reviewer | Review the PR body (shows changelog section), approve, merge |
| 4 | CI | `publish-release.yml` auto-fires: creates tag `v0.5.0` → lint → test → mirror + GitHub Release → Slack |
| 5 | Developer | Verify the version is installable: `go install github.com/adguardteam/go-webext@v0.5.0` |
<!-- markdownlint-disable MD029 -->

### Pipeline Stages

```text
Prepare release (manual)
     │
     ▼
  PR opens (CHANGELOG finalized)
     │
     ▼
  Merge PR
     │
     ▼
  Publish release (auto)
     │
     ▼
  tag → test → mirror-and-release → notify
```
<!-- markdownlint-enable MD029 -->

## Notifications

Successful releases post to Slack via the shared
`AdGuardSoftwareLimited/actions/actions/slack` action.

| Parameter | Value |
|---|---|
| **Channel** | `#adguard-extension-vcs` |
| **Product name** | `@adguard/go-webext` |
| **Message** | `released` |

Slack notification failures are non-blocking — the release continues
even if Slack is unreachable.

## Environment Variables

This tool requires store API credentials at **runtime** (not at release time).
See the [README.md](README.md#environment-variables) for the full list of
environment variables for Chrome, Firefox, and Edge stores.

## Troubleshooting

**Release pipeline fails with "No released version found in
CHANGELOG.md"**

The `publish-release.yml` workflow expects `CHANGELOG.md` to follow
keepachangelog format with bracket version headings
(`## [X.Y.Z] - date`). Ensure the latest version heading matches this
format.

**Tag creation fails**

Check that `CHANGELOG.md` has a `## [Unreleased]` section at the top.
The `create-release-pr` workflow requires this to finalize the
changelog.

**Re-running a failed release**

If `publish-release.yml` fails after the tag was created, go to
**Actions → Publish release → Run workflow** and enter the ref (e.g.,
`v0.5.0` or a commit SHA). This manually triggers the release pipeline.
