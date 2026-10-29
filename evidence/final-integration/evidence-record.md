# Final integration evidence

**Student number:** 100874830

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Final submission tag:** `final-cw1`

**Final tagged commit:** `e4f5f141c94a9b1e66b3189d05365619f9c50466`

**Environment:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `sh scripts/check.sh`
- `go test -race ./... -count=1 -timeout 30s`
- `go mod verify`
- `git diff --check`
- HTTP requests to `/healthz` and `/v1/status` on Node A and Node B
- POST request to register Node B with Node A at `/v1/peers`
- GET request to `/v1/remote-files?peer_id=node-b`
- GET request to Node A's `/v1/files`
- GET request to `/v1/remote-files?peer_id=node-b` while Node B was unavailable

## Observed results

- Both nodes returned HTTP 200 from `/healthz`, with their correct node IDs and an `ok` status.
- Both nodes returned HTTP 200 from `/v1/status`, with a `healthy` status.
- Registering Node B returned HTTP 201.
- Node A retrieved Node B's remote file metadata with HTTP 200. The response contained `diagram.txt` and `notes.txt`.
- Node A's local `/v1/files` endpoint returned HTTP 200 and metadata for `alpha.txt`.
- The automated check script passed, including the build, test suite and `go vet`.
- The race-enabled test suite passed, all Go modules verified, and `git diff --check` reported no errors.

## Failure case and diagnosis

While Node B was unavailable, Node A's remote-files endpoint returned HTTP 503 with `{"error":"peer_unavailable"}`. The request utility then exited with status 1 because it expected HTTP 200 by default. Node A's local file endpoint continued to return HTTP 200. This shows that a remote peer failure is reported clearly without preventing the local file endpoint from responding.

## Design decision and alternative

The implementation uses HTTP for the client-facing API and gRPC with Protocol Buffers for remote file metadata. Node A looks up the registered peer and requests its file metadata through the peer's gRPC address. An alternative would be to use HTTP and JSON for remote metadata as well. The current approach demonstrates the required remote procedure call interface.

## Remaining limitations

The current implementation does not provide authentication or encryption. File contents are not transferred, only metadata. Remote requests can fail when a peer is unavailable, and the file catalogues are held in memory rather than being synchronised through persistent storage.

## Sources and AI-use disclosure

I consulted the coursework guidance, supplied project scaffold and official Go, gRPC and Protocol Buffers documentation. ChatGPT assisted with explanations and troubleshooting. I ran the listed checks and live requests and recorded their observed results.

## Additional live integration checks — 29 October 2026

- **Duplicate peer registration:** Re-registering Node B with Node A returned HTTP 409 with `duplicate_peer`.
- **Reverse peer registration:** Registering Node A with Node B returned HTTP 201. Node B retrieved Node A's `alpha.txt` metadata through `/v1/remote-files?peer_id=node-a`, returning HTTP 200.
- **TCP messaging:** Node A sent `tcp-check-001` to Node B on TCP port 7002. Node B accepted the message and its inbox contained the matching request ID and text. Node B then sent `tcp-check-002` to Node A on port 7001; Node A accepted it and recorded it in its inbox.
- **UDP messaging:** Node A sent `udp-check-001` to Node B on UDP port 7102. Node B accepted the message and its inbox contained the matching request ID and text. Node B then sent `udp-check-002` to Node A on port 7101; Node A accepted it and recorded it in its inbox.
- **Status validation:** Node A's status was changed from `healthy` to `degraded` using POST `/v1/status`, returning HTTP 200. Malformed JSON returned HTTP 400 with `invalid_json`; an invalid status value returned HTTP 400 with `invalid_status`; an oversized request returned HTTP 413 with `body_too_large`. A subsequent GET confirmed the status remained `degraded`.
- **Peer outage and recovery:** With Node B stopped, Node A's `/healthz` and `/v1/files` continued to return HTTP 200. Its remote-files request returned HTTP 503 with `peer_unavailable`. Node B was then restarted and its health endpoint returned HTTP 200.
- **Occupied-port startup failure:** Starting a test node using the already occupied HTTP address `127.0.0.1:8081` produced a clear `address already in use` error and exited unsuccessfully, as expected.
- **Listener shutdown and restart:** After Node B was stopped, checks for its HTTP, TCP, gRPC and UDP listening ports showed no listener output. After restarting Node B, its HTTP, TCP, gRPC and UDP listeners were present again.
