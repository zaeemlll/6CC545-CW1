# Weekly evidence record

**Student number:** 100874830

**Week:** Week 1

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week01`

**Commit hash:** `668916074295da3bf61f1784a89ba14838002435`

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `go version`
- `go test ./internal/config -run "^TestWeek01_" -v`
- `go test ./internal/config -v -count=1`
- `go test ./internal/config -run "TestWeek01_InvalidOptionalAddress" -v -count=1`
- `go test ./... -count=1 -timeout 20s`
- `go vet ./...`
- `sh scripts/run-node-a.sh`
- `go run ./cmd/request -url http://127.0.0.1:8081/healthz`
- `sh scripts/run-node-b.sh`
- `go run ./cmd/request -url http://127.0.0.1:8082/healthz`
- `sh scripts/run-node-a.sh` while Node A was already running
- `go run ./cmd/request -url http://127.0.0.1:8081/healthz` while the original Node A was running
- `go run ./cmd/request -url http://127.0.0.1:8081/healthz` after Node A was stopped

## Observed results

The targeted Week 1 configuration tests passed: `TestWeek01_Config` and `TestWeek01_UnknownFlag`.

The full configuration package test run also passed. It included `TestSmoke_ParseHelp`, `TestWeek01_Config`, and `TestWeek01_UnknownFlag`.

The targeted Week 1 configuration tests passed, including `TestWeek01_Config` and `TestWeek01_UnknownFlag`. A separate check for `TestWeek01_InvalidOptionalAddress` reported that there were no matching tests to run, so it is not recorded as a passed test.



# Week 2 evidence record

**Student number:** 100874830

**Week:** Week 2

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week02` (will identify the completed Week 2 checkpoint)

**Implementation commit hash:** `d4d10e29e5c65af1fa8bfeb08655b5edea7e68cf`

**Commit hash:** *Add the final Week 2 checkpoint commit hash.*

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `go test ./internal/httpapi -run '^TestWeek02_' -v -count=1`
- `go test ./... -count=1 -timeout 20s`
- `go vet ./...`
- Live HTTP requests to test GET and POST `/v1/status` and GET `/healthz`.

## Observed results

- All Week 2 HTTP API tests passed, including the exact 4096-byte request-body limit test.
- The complete Go test suite passed.
- `go vet ./...` completed without reported problems.
- GET `/v1/status` returned HTTP 200 with `{"status":"healthy"}` initially.
- POST updated the status to `degraded`, and a subsequent GET returned the updated status.
- Malformed JSON returned HTTP 400 with `{"error":"invalid_json"}`.
- A GET after the invalid request confirmed the stored status remained `degraded`.
- GET `/healthz` continued to return HTTP 200.

## Failure case and diagnosis

Malformed JSON was rejected with HTTP 400. The stored status did not change because the request was rejected before the update was applied.

## Design decision and alternative

The status is stored in a concurrency-safe state object. Requests are checked for the correct media type, body-size limit and valid JSON before the status is updated. An alternative would be to accept requests without these checks, but that could allow invalid input to reach the application.

## Remaining limitations

The API is intended for local coursework testing and does not include authentication. Duplicate JSON keys are not explicitly rejected.

## Sources and AI assistance

The Week 2 student workbook and relevant Go documentation were used. ChatGPT assisted with explanations and implementation troubleshooting. The tests and live HTTP requests were run to check the implementation.




# Week 3 evidence record

**Student number:** 100874830

**Week:** Week 3

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Checkpoint tag:** `week03` (to be created after final verification)

**Implementation commit hash:** `019be06a7053a9d635a1ec4d860ca620f41f893e`

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

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

## Observed results

- Model, registry, catalogue, and peer API tests passed.
- The full test suite passed, including the race detector.
- `go vet` reported no problems.
- Both node startup scripts passed shell syntax checks.
- Node A and Node B returned HTTP 200 from `/v1/files`.
- Registering Node B returned HTTP 201; repeating the registration returned HTTP 409 with `duplicate_peer`.
- `/v1/peers` listed the registered peer, and `/v1/status` continued to return `healthy`.

## Failure case and diagnosis

Registering the same peer ID twice returned HTTP 409 Conflict with `{"error":"duplicate_peer"}`. This is expected behaviour because peer IDs must be unique.

## Design decision and alternative

The peer registry uses a map protected by `sync.RWMutex` so concurrent requests can safely read and add peers. The catalogue validates replacement data before updating its stored list, so invalid data does not replace the existing catalogue. An alternative would be to use a separate database, but an in-memory structure is sufficient for this checkpoint.

## Remaining limitations

Peer registration stores peer information but does not confirm that the peer is reachable. The catalogue stores metadata, not file contents, and the service has no authentication. It is intended for local coursework testing on loopback addresses.

## Sources and AI assistance

Used the Week 3 student workbook, supplied coursework scaffold, and Go documentation. ChatGPT assisted with explanations and troubleshooting. I ran the commands and checked the observed results.
