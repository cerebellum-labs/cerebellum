# Releasing

Each package is versioned and released on its own, with `<package>/vX.Y.Z` tags and its own `CHANGELOG.md`. Only the server has a release workflow so far; the agent will follow the same steps (`agent/vX.Y.Z`) once it has code.

## Choosing the version

Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html). While a package is 0.x:

- a change that breaks the agent–server API or the server's configuration raises the minor version: `0.2.3` → `0.3.0`;
- anything else raises the patch version: `0.2.3` → `0.2.4`.

Versions only grow: the release workflow refuses a version that isn't above the latest release.

## Release the server

### 1. Changelog

In a pull request, turn `## [Unreleased]` in `packages/server/CHANGELOG.md` into the new version, start an empty `## [Unreleased]` above it and update the links at the bottom:

```markdown
## [Unreleased]

## [0.2.0] - 2026-10-29

### Added

- …

[Unreleased]: https://github.com/cerebellum-labs/cerebellum/compare/server/v0.2.0...HEAD
[0.2.0]: https://github.com/cerebellum-labs/cerebellum/compare/server/v0.1.0...server/v0.2.0
```

For the first release, the version link points at the tag itself: `[0.1.0]: https://github.com/cerebellum-labs/cerebellum/releases/tag/server/v0.1.0`.

Commit it as `chore(server): release 0.2.0` and merge. The section becomes the text of the GitHub release.

### 2. Tag

```bash
git checkout main && git pull
```

```bash
make -C packages/server release V=0.2.0
```

`make release` refuses to tag unless you are on `main`, with no local changes, level with `origin/main`, and the changelog has a `## [0.2.0]` section. It then creates the annotated tag `server/v0.2.0` and pushes it. If the push fails, it deletes the local tag, so you can run it again.

### 3. Watch the workflow

**Actions → Release** runs, in order:

1. **Check tag:** the version format, the tagged commit is on `main`, the version is above the latest release, the changelog section exists, and the version isn't already published from another commit.
2. **Server:** lint, tests and the vulnerability check, then the image for `linux/amd64` and `linux/arm64`, built without cache and tagged `0.2.0`, `0.2` and `latest`.
3. **Attest image:** signed build provenance, so anyone can check which workflow and commit built the image.
4. **GitHub release:** `server v0.2.0`, with the changelog section and the image name, marked Latest.

Releases run one at a time, in the order their tags were pushed.

### 4. Check the result

- The release is on the [Releases](https://github.com/cerebellum-labs/cerebellum/releases) page.
- The image runs and reports its version in the `server starting` log line:

  ```bash
  docker run --rm -p 8080:8080 ghcr.io/cerebellum-labs/cerebellum-server:0.2.0
  ```

- The provenance verifies:

  ```bash
  gh attestation verify oci://ghcr.io/cerebellum-labs/cerebellum-server:0.2.0 --repo cerebellum-labs/cerebellum
  ```

## Release candidates

To try a version before releasing it:

```bash
make -C packages/server release V=0.2.0-rc.1
```

A release candidate needs no changelog section: its notes are the current `## [Unreleased]`. Its image is tagged `0.2.0-rc.1` only, it never moves `latest` or `0.2`, and its GitHub release is marked as a pre-release. Release `0.2.0` itself the usual way once the candidate is good.

## When something goes wrong

| What happened | What to do |
| --- | --- |
| `make release`: no `## [X.Y.Z]` section | Merge the changelog pull request first, then `git pull` |
| `make release`: `main differs from origin/main` | `git pull`, or push your commits through a pull request first |
| `make release`: the tag push was rejected | The local tag is already gone; fix the cause (access, network) and run it again |
| Check tag failed, or Server failed before the image was pushed | Nothing was published. Fix it on `main` and release the next patch version. A maintainer may instead delete the unused tag and tag again |
| Attest image or GitHub release failed | **Re-run failed jobs** on the run: the published image is detected and not rebuilt |
| A run was cancelled while waiting | Several tags were pushed at once and GitHub kept only the newest waiting run. **Re-run** the cancelled one |
| The released code is wrong | Release the next patch version. Never move or reuse a released tag |
