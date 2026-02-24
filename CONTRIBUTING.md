# Contributing

## Workflow

1. Open an issue describing the problem or proposal.
2. Create a branch from `main`.
3. Keep protocol, bindings, and conformance artifacts aligned.
4. Submit a pull request with:
- motivation
- normative changes (if any)
- compatibility impact
- tests/validation notes

## Spec and Runtime Change Rules

- Use RFC 2119 language only for normative requirements.
- Any core semantic change in `rfc/MIG-0001.md` must update:
- `proto/mig/v0_1/mig.proto`
- `openapi/mig.v0.1.yaml`
- `conformance/CONFORMANCE-CHECKLIST.md`
- Interop changes must update `docs/interop/MCP-MIG-MAPPING.md`.
- Gateway API changes must update related OpenAPI documents under `openapi/`.

## Governance

MIG uses the MIG-EP process defined in `docs/GOVERNANCE.md`.
Use `rfc/TEMPLATE.md` for proposals.

## Versioning

- Protocol: pre-1.0 policy where breaking changes increment minor (`0.1` -> `0.2`).
- Runtime: semver (`migd` is currently `0.x`).

## Testing

Run before opening a PR:

```bash
go test ./core/... ./adapters/mcp ./conformance/harness ./sdk/go/...
```
