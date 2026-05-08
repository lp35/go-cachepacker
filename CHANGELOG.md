# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v1.0.0] - 2026-05-10

### Added

- Create Go module ZIP archives from a local VCS repository, compatible with the Go module proxy protocol.
- Automatic output directory: defaults to `${GOMODCACHE}/cache/download/<MODULE>/@v` when `-out` is not specified.
- Support for both regular version tags (e.g. `v1.2.3`) and pseudo-versions (e.g. `v0.0.1-pre.0.20260417064548-abcdefabcdef`); the git ref is resolved automatically.
- Support for absolute and relative repository paths via `-repo`.
- CLI help with usage description, flag documentation, and example.
