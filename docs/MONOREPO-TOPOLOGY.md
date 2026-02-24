# Monorepo Topology

This repository follows a monorepo-with-overlays model.

- `core/` is the OSS protocol/runtime surface.
- `pro/` and `cloud/` define commercial overlay surfaces.
- `adapters/`, `sdk/`, and `conformance/` are shared supporting assets.

## Release Strategy

- Protocol artifacts remain MIG `v0.1` compatible.
- Runtime follows semver (`0.x` pre-GA).
- CI is split into `core-ci`, `pro-ci`, and `cloud-ci` lanes.

## Public Mirror Guidance

The public mirror should include:
- `core/`
- `adapters/`
- `conformance/`
- `sdk/go/`, `sdk/python/`, `sdk/typescript/`
- protocol specs and RFC docs
