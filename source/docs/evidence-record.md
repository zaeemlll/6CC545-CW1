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

- The targeted Week 1 configuration tests passed. These included `TestWeek01_Config` and `TestWeek01_UnknownFlag`.
- The full configuration package test run also passed. It included `TestSmoke_ParseHelp`, `TestWeek01_Config` and `TestWeek01_UnknownFlag`.
- The test for `TestWeek01_InvalidOptionalAddress` did not run because no matching test was found. I have not recorded it as a passed test.
- The full project test run passed, and `go vet ./...` completed without reporting problems.
- Requests to the health endpoints on Node A (`127.0.0.1:8081`) and Node B (`127.0.0.1:8082`) returned HTTP 200, according to the recorded results.

## Failure case and diagnosis

The configuration checks do not check whether a port is already in use. When the server tries to use a port that another process is using, the operating system returns an error and the server cannot start on that address. This means the configuration can pass its checks but the server can still fail to start.

## Design decision and alternative

The code keeps configuration checks, HTTP request handling, and server startup and shutdown in separate components. This makes the code easier to understand, test and maintain. Command-line flags allow the same program to run as different nodes, each with its own ID, HTTP address and shutdown timeout.

The configuration checks validate the node ID, loopback IP address, port range and shutdown timeout before the server starts. The `/healthz` endpoint returns the node ID and an `ok` status in JSON. It accepts GET requests, returns `405 Method Not Allowed` with an `Allow: GET` header for other methods, and returns `404 Not Found` for unknown paths.

One alternative would be to put the configuration checks, HTTP handling and startup code into one component. I chose to keep them separate because this makes each part easier to test and update.

## Remaining limitations

The health endpoint only checks whether the local HTTP handler responds. It does not show whether the complete distributed service is working. The server uses a loopback address by default, which limits access from other machines. However, it does not provide authentication or encryption, so other programs on the same computer may still access it.

The server also supports graceful shutdown, but the result depends on the timeout setting and how long active requests take to finish. If the timeout expires too early, some requests may not finish. The current implementation provides basic node configuration and health monitoring, but it does not yet support communication between nodes or manage shared state. More work is needed to add these features and suitable security measures.

## Sources and AI-use disclosure

I have consulted the official Go documentation on command line flags, HTTP servers and graceful shutdown. I have also got partial support from artificial intelligence.


# Week 2 Evidence Record

**Student number:** 100874830

**Week:** Week 2

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week02`

**Commit hash:** `d4d10e29e5c65af1fa8bfeb08655b5edea7e68cf`

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `go test ./internal/httpapi -run '^TestWeek02_' -v -count=1`
- `go test ./... -count=1 -timeout 20s`
- `go vet ./...`
- Live HTTP GET and POST requests to `/v1/status`.
- A live HTTP GET request to `/healthz`.
- A request containing malformed JSON to test input validation.

## Observed results

- All targeted Week 2 HTTP API tests passed, including the exact 4096-byte request-body limit test.
- The complete Go test suite passed.
- `go vet ./...` completed without reporting problems.
- An initial GET request to `/v1/status` returned HTTP 200 with `{"status":"healthy"}`.
- A POST request updated the status to `degraded`, and a subsequent GET returned the updated status.
- Malformed JSON returned HTTP 400 with `{"error":"invalid_json"}`.
- A subsequent GET confirmed that the stored status remained `degraded` after the invalid request.
- GET `/healthz` continued to return HTTP 200.

## Failure case and diagnosis

A request containing malformed JSON was rejected with HTTP 400 and the error `invalid_json`. The request was rejected before the status update was applied, so the previously stored status remained `degraded`. This demonstrates that the API validates JSON input before modifying the stored status.

## Design decision and alternative

The REST interface uses HTTP methods and JSON to exchange data between clients and a node. The status is stored in a concurrency-safe state object. Requests are checked for the appropriate content type, body-size limit and valid JSON before the status is updated. This helps prevent invalid requests from modifying application state and supports safe access to shared status data.

An alternative would be to accept requests without validating their content or to use a different data format. However, accepting unvalidated input could allow malformed or unsuitable data to reach the application. JSON was chosen because it is readable and widely supported by HTTP clients.

## Remaining limitations

The API is intended for local coursework testing and does not include authentication or encryption. Consequently, it would need additional security measures before being exposed to an untrusted network. Request validation improves reliability but does not protect against every form of misuse. Duplicate JSON keys are not explicitly rejected. The status is stored in memory, so it is not persistent across application restarts.

## Sources and AI assistance

The Week 2 student workbook and relevant Go documentation were used. ChatGPT assisted with explanations and implementation troubleshooting. The tests and live HTTP requests were run to check the implementation, and the observed results were recorded in the project evidence.





# Week 3 evidence record

**Student number:** 100874830

**Week:** Week 3

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Checkpoint tag:** `week03`

**Implementation commit hash:** `019be06a7053a9d635a1ec4d860ca620f41f893e`

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

## Observed results

- The same-peer concurrency test passed: out of 50 concurrent attempts to register the same peer ID, exactly one succeeded and 49 returned duplicate errors.
- The architecture diagram shows Node A and Node B as separate processes with separate in-memory registries and catalogues.
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

# Week 4 evidence record

**Student number:** 100874830

**Week:** Week 4

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Checkpoint tag:** `week04` (to be created after verification).

**Implementation commit hash:** `71499ae4952ae27b4d76c571961e8132c161ef58`

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `go test ./internal/ipc -run "^TestWeek04_" -v -count=1`
- `go test ./... -count=1 -timeout 20s`
- `go test -race ./... -count=1 -timeout 30s`
- `go vet ./...`
- Live TCP and UDP client requests between Node A and Node B.
- Live GET requests to `/v1/messages` on both nodes.

## Observed results

- Targeted Week 4 IPC tests passed, covering TCP and UDP round trips, oversized TCP messages, UDP timeout/drop behaviour, cancelled clients, fragmented TCP frames, frame-size limits, and invalid messages.
- The full Go test suite passed.
- The race detector passed without reported race warnings.
- `go vet ./...` completed without reported problems.
- TCP and UDP messages were accepted in both directions between Node A and Node B.
- Both `/v1/messages` endpoints returned HTTP 200 and showed received messages.
- With Node B stopped, the TCP client reported connection refused. The UDP client also reported connection refused while waiting for a response in this local test.

## Failure case and diagnosis

Stopping Node B prevented successful messaging. TCP reported connection refused when the client attempted to connect. The UDP client reported connection refused while waiting for a reply in this local environment. UDP does not guarantee delivery, and behaviour when a destination is unavailable can depend on the operating system.

## Design decision and alternative

Messages use validated JSON. TCP uses a four-byte length prefix to identify message boundaries. UDP sends each message as a datagram and does not guarantee delivery. Both transports return acknowledgements when the receiver accepts a message. Accepted messages are stored in a bounded in-memory inbox exposed through `GET /v1/messages`. A database or durable message queue could retain messages across restarts, but an in-memory inbox is sufficient for this checkpoint.

## Remaining limitations

The inbox is in memory and is not preserved after a node restarts. UDP can lose or reorder messages. The local teaching service does not include authentication and should not be exposed to an untrusted network.

## Sources and AI assistance

Used the Week 4 coursework guidance, supplied project scaffold, and Go documentation. ChatGPT assisted with explanations and troubleshooting. I ran the tests and live requests and checked the observed results.

# Week 5 evidence record

**Student number:** 100874830

**Week:** Week 5

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Checkpoint tag:** `week05`.

**Implementation commit hash:** `4ae1cae`.

**Go version / OS:** `go1.27.1 darwin/amd64`

**Protobuf compiler:** `libprotoc 36.2`

**Protobuf generators:** `protoc-gen-go v1.36.6`; `protoc-gen-go-grpc 1.5.1`

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

## Observed results

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
- RPC tests passed for remote listing, unavailable peers, cancelled contexts, and an empty catalogue. HTTP validation tests passed for the wrong method, missing or invalid peer ID, and unknown peer.

## Design decision and alternative

The nodes exchange file metadata through a gRPC `FileService`. The HTTP endpoint on Node A looks up the registered peer and requests metadata from that peer's gRPC address. The remote request has a timeout, and failures are mapped to HTTP error responses. The same catalogue used by the local file endpoint is exposed through the gRPC service. An alternative would be to exchange metadata through HTTP, but this checkpoint uses gRPC and Protocol Buffers.

## Remaining limitations

The service is intended for local coursework testing. It uses insecure gRPC transport for loopback addresses and does not implement authentication or encryption. Peer registration does not guarantee that the peer is online, so remote requests can fail when a peer is unavailable. File contents are not transferred; only metadata is returned.

## Sources and AI assistance

Used the supplied Week 5 coursework scaffold, dependency notes, and Go/gRPC/Protocol Buffers tooling. ChatGPT assisted with explanations and troubleshooting. The commands, tests, and live requests were run to check the implementation.
