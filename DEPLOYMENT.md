# Deployment

`go-webext` is a stateless CLI tool distributed as a Go module. There is no
server or infrastructure to deploy. "Deployment" means publishing a new version
that users can install via `go install`.

## Release Process

### 1. Prepare the Release

1. Ensure all changes are merged into the main branch.
2. Run the full verification suite:

    ```bash
    make test
    make lint
    ```

3. Update `CHANGELOG.md` with the new version entry following
   [Keep a Changelog](https://keepachangelog.com/) format.
4. Commit the changelog update.

### 2. Create a Tag

Tag the release commit with a semantic version prefixed by `v`:

```bash
git tag v0.4.1
```

Follow [Semantic Versioning](https://semver.org/):

- **Major** (`v1.0.0`) — breaking changes to CLI flags, environment variables,
  or behavior.
- **Minor** (`v0.4.0`) — new commands, flags, or store API support.
- **Patch** (`v0.4.1`) — bug fixes and minor improvements.

### 3. Push the Tag

```bash
git push origin v0.4.1
```

### 4. Create a GitHub Release

1. Go to the repository's **Releases** page on GitHub.
2. Click **Draft a new release**.
3. Select the tag you just pushed.
4. Set the release title to the tag name (e.g., `v0.4.1`).
5. Copy the relevant `CHANGELOG.md` section into the release description.
6. Publish the release.

### 5. Verify

After publishing, confirm the version is installable:

```bash
go install github.com/adguardteam/go-webext@v0.4.1
```

## CI/CD

Bamboo CI (`bamboo-specs/bamboo.yaml`) runs **Test** and **Lint** stages on
every push and pull request. There is no automated release pipeline — releases
are created manually via GitHub.

## Environment Variables

This tool requires store API credentials at **runtime** (not at release time).
See the [README.md](README.md#environment-variables) for the full list of
environment variables for Chrome, Firefox, and Edge stores.
