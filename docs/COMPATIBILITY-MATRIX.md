# Compatibility Matrix

| Runtime Version | MIG Protocol | Core Profile | Streaming | Evented | Pro APIs | Cloud APIs |
| --- | --- | --- | --- | --- | --- | --- |
| `migd 0.1.x` | `0.1` | Yes | Yes | Yes | Yes | Yes |

## Notes

- Core and Pro APIs are tested from the same runtime commit and released together.
- Cloud control-plane APIs are pinned to tested runtime versions.
- Breaking protocol changes require minor protocol version bump (`0.1` -> `0.2`).
