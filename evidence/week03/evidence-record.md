# Week 3 Evidence Record

**Student number:** 100874830
**Repository:** https://github.com/zaeemlll/6CC545-CW1
**Checkpoint tag:** `week03`
**Checkpoint commit:** `ad11ca84a6b3fc0906e222d07892a88f50ac83ff`

## Aim
Add peer registration and a local file-metadata catalogue.

## Observed behaviour previously recorded
- Successful peer registration returned HTTP 201.
- Re-registering a peer with the same ID returned HTTP 409.
- The `/v1/peers` endpoint listed registered peers.
- The `/v1/files` endpoint returned local file metadata.
- A concurrency test with 50 attempts for the same peer ID produced one success and 49 duplicate errors.

## Design decision
The peer registry and catalogue maintain separate in-memory state. The registry uses locking to protect concurrent access, and the nodes have separate state.

## Documentation
See `source/docs/week03-architecture.md` for the architecture, component descriptions and limitations.

## Limitations
The catalogue stores metadata, not file contents. Peer registration does not by itself guarantee that a peer is reachable.
