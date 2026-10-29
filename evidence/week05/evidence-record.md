# Week 5 Evidence Record

**Student number:** 100874830
**Repository:** https://github.com/zaeemlll/6CC545-CW1
**Checkpoint tag:** `week05`
**Checkpoint commit:** `4ae1cae06ca6195d5a872563143208c4064bc429`

## Aim
Implement gRPC communication using Protocol Buffers to request remote file metadata.

## Observed behaviour previously recorded
- Registering Node B returned HTTP 201.
- A remote-files request returned HTTP 200 and metadata including `diagram.txt` and `notes.txt`.
- When the remote gRPC service was unavailable, the HTTP endpoint returned HTTP 503 with `peer_unavailable`.

## Design decision
The project uses a gRPC service contract defined with Protocol Buffers. The HTTP endpoint exposes the remote metadata request to a client.

## Documentation
See `source/docs/api-and-protocols.md`, section "Week 5 — gRPC Remote File Metadata".

## Limitations
The feature retrieves metadata, not the actual file contents. The unavailable-peer result is an expected failure case.
