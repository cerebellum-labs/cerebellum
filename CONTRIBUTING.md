# Contributing

Thanks for helping. The repository layout is in the [README](README.md#repository-layout); each package's README will say how to build and run it.

## Workflow

1. Branch from `main`.
2. Keep a pull request to one change and say what and why in its description.
3. Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) with the package as scope: `feat(agent): ...`, `fix(server): ...`.
4. A change to the agent–server API updates the agent and the server in the same pull request.

## Secrets, models and data

- No keys or tokens in the repository; they go in environment variables.
- Do not commit model weights, datasets, frames or SQLite files. Document where to download them instead.
- Every third-party model, dataset or library with its own license goes into [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Check that the license allows use from an MIT project before adding it.

## CI and images

- Every push to any branch runs CI. It checks the packages the branch changes compared with `main` (a change under `.github/` checks all of them) and ends in `ci-ok`, the one check to require on `main`.
- A branch builds the server image, without publishing it, only when it changes `packages/server/Dockerfile`, `packages/server/.dockerignore` or `.github/`.
- A merge to `main` that changes the server since the current `main` image checks it and publishes a new image as `main` and `sha-<commit>`.
- For an image of a branch, run **Actions → CI → Run workflow** on it: it is published as `br-<branch>` and `sha-<commit>`.
- CI runs on pushes to this repository, so a pull request from a fork gets no checks; a maintainer pushes its branch here to run them before merging.
- The server image is `ghcr.io/cerebellum-labs/cerebellum-server`, for `linux/amd64` and `linux/arm64`. `latest` and `X.Y` only come from releases.
- `make lint test` in `packages/server` runs the same checks locally. CI uses golangci-lint v2.15.0 (`.github/workflows/server.yml`); use the same version. `make test` runs the unit tests; the tests against PostgreSQL carry the `integration` build tag and run with `make db test-integration` (PostgreSQL 18 in Docker). After changing `internal/db/queries` or `internal/db/migrations`, run `make generate`.
- The dashboard (`packages/dashboard`) uses Node.js 24 and pnpm. `pnpm install`, then `pnpm lint`, `pnpm typecheck` and `pnpm build` run the same checks as CI. It is deployed by its host from `main`, with no images or releases.

## Changelog and releases

Each package is versioned and released on its own. Its `CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/): changes users notice go under `## [Unreleased]` in the pull request that makes them.

How to release a package, and what to do when a release fails: [RELEASING.md](RELEASING.md).
