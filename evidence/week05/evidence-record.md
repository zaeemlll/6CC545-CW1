# Week 5 evidence record

**Student number:** 100874830

**Week:** Week 5

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week05`

**Commit hash:** `4ae1cae06ca6195d5a872563143208c4064bc429`

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `sh scripts/generate.sh`
- `go build ./...`
- `go test ./internal/rpcfiles -run '^TestWeek05_' -v -count=1`
- `go test ./internal/httpapi ./internal/rpcfiles -count=1`
- `go test ./... -count=1 -timeout 20s`
- `go test -race ./... -count=1 -timeout 30s`
- `go vet ./...`
- `go mod verify`
- `git diff --check`
- `sh scripts/check.sh`
- Live HTTP requests to register Node B and retrieve `/v1/remote-files?peer_id=node-b`.
- Live HTTP requests to check `/v1/status` and `/v1/files` while Node B was unavailable.

## Observed results (do not paste expected examples as results)

- Protobuf bindings were generated using the pinned Go generators and `protoc` 36.2.
- The full Go test suite passed.
- The race detector passed without reported race warnings.
- `go vet ./...` reported no problems.
- `go mod verify` reported that all modules were verified.
- `git diff --check` reported no whitespace errors.
- `sh scripts/check.sh` passed.
- Node B registration returned HTTP 201.
- Node A's remote-files endpoint returned HTTP 200 and Node B's file metadata, including `diagram.txt` and `notes.txt`.
- When Node B was stopped, the remote-files endpoint returned HTTP 503 with `{"error":"peer_unavailable"}`. Node A's status endpoint and local file listing continued to return HTTP 200.
- RPC tests passed for remote listing, unavailable peers, cancelled contexts and an empty catalogue.
- HTTP validation tests passed for the wrong method, missing or invalid peer ID, and unknown peer.

## Failure case and diagnosis

When Node B was unavailable, Node A could not retrieve remote metadata. The remote-files endpoint returned HTTP 503 with `peer_unavailable`, while Node A's own status and local file listing continued to work. This demonstrates that the remote operation can fail without preventing the local endpoints from responding.

## Design decision and alternative

The nodes exchange file metadata through a gRPC `FileService`, defined using Protocol Buffers. The `.proto` contract describes the service and the structure of its request and response messages. Generated Go bindings connect the contract to the client and server.

The HTTP endpoint on Node A looks up the registered peer and requests metadata from that peer's gRPC address. The remote request has a timeout, and failures are mapped to HTTP error responses. The same catalogue used by the local file endpoint is exposed through the gRPC service.

An alternative would be to exchange metadata through an HTTP endpoint using JSON. This checkpoint uses gRPC and Protocol Buffers to provide a defined service contract and structured messages.

## Remaining limitations

The service is intended for local coursework testing. It uses insecure gRPC transport for loopback addresses and does not implement authentication or encryption. Peer registration does not guarantee that the peer is online, so remote requests can fail when a peer is unavailable. File contents are not transferred; only metadata is returned.

The recorded tests cover remote listing, unavailable peers, cancelled contexts and an empty catalogue. HTTP validation tests cover the listed HTTP cases. Direct gRPC invalid-request handling should only be claimed as tested if a corresponding RPC-level test and result are available.

## Sources and AI-use disclosure

I consulted the supplied Week 5 coursework scaffold, dependency notes, the official Protocol Buffers documentation, Go documentation and gRPC guidance on deadlines. ChatGPT assisted with explanations and troubleshooting. I ran the commands, tests and live requests to check the implementation.