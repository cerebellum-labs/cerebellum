# Changelog

All notable changes to the Cerebellum agent are listed here, in the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) and are released as `agent/vX.Y.Z` tags; 0.x until the agent–server API settles.

## [Unreleased]

### Added

- The agent library: rules, a decision journal and queue in SQLite (WAL), a background thread that asks the server for irreversible decisions and syncs everything else, corrections, and `on_decision`, `on_correction` and `on_connection` callbacks. It follows the server's [Agent API](../server/AGENT_API.md).

[Unreleased]: https://github.com/cerebellum-labs/cerebellum/commits/main/packages/agent
