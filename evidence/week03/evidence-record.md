# Week 3 evidence record

**Student number:** 100874830

**Week:** Week 3

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week03`

**Commit hash:** `ad11ca84a6b3fc0906e222d07892a88f50ac83ff` (checkpoint tag target; implementation commit: `019be06a7053a9d635a1ec4d860ca620f41f893e`)

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `go test ./internal/state -run '^TestWeek03_' -v -count=1`
- `go test ./internal/model -count=1`
- `go test ./internal/state -run '^TestWeek03_Registry$' -v -count=1`
- `go test ./internal/state -run '^TestWeek03_ConcurrentRegistry$' -v -count=1`
- `go test ./internal/state -run '^TestWeek03_InvalidPeer$' -v -count=1`
- `go test ./internal/state -run '^TestWeek03_Catalog$' -v -count=1`
- `go test ./internal/httpapi -run '^TestWeek03_PeerAPI$' -v -count=1`
- `go test ./... -count=1 -timeout 20s`
- `go vet ./...`
- `go test -race ./... -count=1 -timeout 30s`
- `sh -n scripts/run-node-a.sh`
- `sh -n scripts/run-node-b.sh`

## Observed results (do not paste expected examples as results)

- The same-peer concurrency test passed: out of 50 concurrent attempts to register the same peer ID, exactly one succeeded and 49 returned duplicate errors.
- The architecture diagram shows Node A and Node B as separate processes with separate in-memory registries and catalogues.
- Model, registry, catalogue and peer API tests passed.
- The full test suite passed, including the race detector.
- `go vet` reported no problems.
- Both node startup scripts passed shell syntax checks.
- Node A and Node B returned HTTP 200 from `/v1/files`.
- Registering Node B returned HTTP 201. Repeating the registration returned HTTP 409 with `duplicate_peer`.
- `/v1/peers` listed the registered peer, and `/v1/status` continued to return `healthy`.

## Failure case and diagnosis

Registering the same peer ID twice returned HTTP 409 Conflict with `{"error":"duplicate_peer"}`. This is expected behaviour because peer IDs must be unique. Rejecting duplicate IDs prevents the registry from accepting two registrations with the same ID.

## Design decision and alternative

The implementation defines node and file metadata and manages peer information through a separate registry. The registry uses a map protected by `sync.RWMutex` so concurrent requests can safely read and add peers. The catalogue validates replacement data before updating its stored list, so invalid data does not replace the existing catalogue.

Separating state management from the communication handlers makes the code easier to test and maintain. An alternative would be to use a separate database, but an in-memory structure is sufficient for this checkpoint.

## Remaining limitations

Peer registration stores peer information but does not confirm that the peer is reachable. The catalogue stores metadata, not file contents, and the service has no authentication. Each node has its own in-memory registry and catalogue, so the information is not automatically synchronised between nodes. Stale records, peer failures and state synchronisation need further consideration.

## Sources and AI-use disclosure

I consulted the Week 3 student workbook, the supplied coursework scaffold and the Go documentation on concurrency and the race detector. ChatGPT assisted with explanations and troubleshooting. I ran the commands and checked the observed results.