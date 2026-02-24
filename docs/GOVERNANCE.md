# MIG Governance

## RFC Process

MIG protocol and profile changes are governed through MIG Enhancement Proposals (MIG-EP).

1. Open an RFC issue describing the motivation and compatibility impact.
2. Submit a proposal using `rfc/TEMPLATE.md`.
3. Mark normative requirements with RFC 2119 language.
4. Include compatibility analysis and conformance impact.
5. Keep review open for at least 7 calendar days.
6. Approval requires one maintainer and one implementer sign-off.

## Compatibility Rules

- MIG pre-1.0 compatibility policy applies: breaking changes increment minor (`0.1` -> `0.2`).
- Every accepted core semantic change MUST update all of:
- `rfc/MIG-0001.md`
- `proto/mig/v0_1/mig.proto`
- `openapi/mig.v0.1.yaml`
- `conformance/CONFORMANCE-CHECKLIST.md`
- Interop changes MUST update `docs/interop/MCP-MIG-MAPPING.md`.

## Conformance and Certification

- Conformance profiles are the baseline anti-fragmentation control.
- `MIG Compatible` certification requires passing automated harness runs for declared profiles.
- Implementations MUST declare profile support and test matrix evidence.

## Product Boundary Policy

- MIG Core remains transport-neutral and open-source.
- Pro and Cloud tiers may add operational controls, but MUST NOT introduce proprietary protocol semantics.
- New commercial features should use additive metadata (`meta.idg.*`) or side-channel admin/control APIs.
