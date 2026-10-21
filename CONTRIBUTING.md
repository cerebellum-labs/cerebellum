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

## Changelog

[CHANGELOG.md](CHANGELOG.md) follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/): changes users notice go under `## [Unreleased]`.
