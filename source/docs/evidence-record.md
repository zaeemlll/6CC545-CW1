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

## Observed results (do not paste expected examples as results)

- The targeted Week 1 configuration tests passed, including `TestWeek01_Config` and `TestWeek01_UnknownFlag`.
- The full configuration package test run passed. It included `TestSmoke_ParseHelp`, `TestWeek01_Config` and `TestWeek01_UnknownFlag`.
- The test for `TestWeek01_InvalidOptionalAddress` did not run because no matching test was found. It has not been counted as a passed test.
- The full project test run passed, and `go vet ./...` completed without reporting problems.
- Requests to Node A (`127.0.0.1:8081`) and Node B (`127.0.0.1:8082`) returned HTTP 200 from `/healthz`, according to the recorded results.

## Failure case and diagnosis

The configuration checks do not confirm whether a port is already in use. When the server attempts to use a port that another process is using, the operating system returns an error and the server cannot start on that address. Therefore, configuration can pass its checks while the server still fails to start.

## Design decision and alternative

The code separates configuration checks, HTTP request handling, and server startup and shutdown into different components. This makes each component easier to understand, test and maintain. Command-line flags allow the same program to run as different nodes, each with its own ID, HTTP address and shutdown timeout.

The configuration checks validate the node ID, loopback IP address, port range and shutdown timeout before the server starts. The `/healthz` endpoint returns the node ID and an `ok` status in JSON. It accepts GET requests, returns `405 Method Not Allowed` with an `Allow: GET` header for other methods, and returns `404 Not Found` for unknown paths.

One alternative would be to put the configuration checks, HTTP handling and startup code into one component. I chose to keep them separate because this makes the program more reusable and each part easier to test and update.

## Remaining limitations

The health endpoint only checks whether the local HTTP handler responds. It does not confirm that the complete distributed service is working. The server uses a loopback address by default, which limits access from other machines. However, it does not provide authentication or encryption, so other local processes may still access it.

The server attempts a graceful shutdown within a configured timeout. If the timeout expires before active requests finish, some requests may not complete. The implementation provides a modular foundation for node configuration and health monitoring, but it does not yet support inter-node communication or shared state management.

## Sources and AI-use disclosure

I consulted the official Go documentation on command-line flags, HTTP servers and graceful shutdown. I also received partial support from artificial intelligence for explanations and troubleshooting.


# Weekly evidence record

**Student number:** 100874830

**Week:** Week 2

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week02`

**Commit hash:** `01f69e614da67e4ecd50e9b6950154c608902f80` (checkpoint tag target; implementation commit: `d4d10e29e5c65af1fa8bfeb08655b5edea7e68cf`)

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `go test ./internal/httpapi -run '^TestWeek02_' -v -count=1`
- `go test ./... -count=1 -timeout 20s`
- `go vet ./...`
- Live HTTP GET and POST requests to `/v1/status`.
- A live HTTP GET request to `/healthz`.
- A request containing malformed JSON to test input validation.

## Observed results (do not paste expected examples as results)

- All targeted Week 2 HTTP API tests passed, including the exact 4096-byte request-body limit test.
- The complete Go test suite passed.
- `go vet ./...` completed without reporting problems.
- An initial GET request to `/v1/status` returned HTTP 200 with `{"status":"healthy"}`.
- A POST request updated the status to `degraded`, and a subsequent GET returned the updated status.
- Malformed JSON returned HTTP 400 with `{"error":"invalid_json"}`.
- A subsequent GET confirmed that the stored status remained `degraded` after the invalid request.
- GET `/healthz` continued to return HTTP 200.

## Failure case and diagnosis

A request containing malformed JSON was rejected with HTTP 400 and the error `invalid_json`. The request was rejected before the status update was applied, so the previously stored status remained `degraded`. This demonstrates that the API validates submitted JSON before modifying the stored status.

## Design decision and alternative

The REST interface uses HTTP methods and JSON to exchange data between a client and a node. GET requests retrieve the current status, while POST requests submit a status update. The status is stored in a concurrency-safe state object.

Requests are checked for the appropriate content type, body-size limit and valid JSON before the status is updated. These checks improve reliability and help prevent invalid requests from changing application state. JSON was chosen because it is readable and widely supported by HTTP clients.

An alternative would be to accept requests without validating their content or to use a different data format. However, accepting unvalidated input could allow malformed or unsuitable data to reach the application.

## Remaining limitations

The API is intended for local coursework testing and does not include authentication or encryption. Additional security measures would be needed before exposing it to an untrusted network. Request validation improves reliability but does not protect against every form of misuse. Duplicate JSON keys are not explicitly rejected. The status is stored in memory, so it is not persistent across application restarts.

## Sources and AI-use disclosure

I consulted RFC 9110 for HTTP semantics and the OWASP REST Security Cheat Sheet for API security guidance. I also received partial AI assistance to review and understand the material.


# Weekly evidence record

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


# Weekly evidence record

**Student number:** 100874830

**Week:** Week 4

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week04`

**Commit hash:** `5f83cac10700f86db11963cb675fa73d5f930f3d` (checkpoint tag target; implementation commit: `71499ae4952ae27b4d76c571961e8132c161ef58`)

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `go test ./internal/ipc -run "^TestWeek04_" -v -count=1`
- `go test ./... -count=1 -timeout 20s`
- `go test -race ./... -count=1 -timeout 30s`
- `go vet ./...`
- Live TCP and UDP client requests between Node A and Node B.
- Live GET requests to `/v1/messages` on both nodes.

## Observed results (do not paste expected examples as results)

- Targeted Week 4 IPC tests passed, covering TCP and UDP round trips, oversized TCP messages, UDP timeout/drop behaviour, cancelled clients, fragmented TCP frames, frame-size limits and invalid messages.
- The full Go test suite passed.
- The race detector passed without reported race warnings.
- `go vet ./...` completed without reported problems.
- TCP and UDP messages were accepted in both directions between Node A and Node B.
- Both `/v1/messages` endpoints returned HTTP 200 and showed received messages.
- With Node B stopped, the TCP client reported connection refused. The UDP client also reported connection refused while waiting for a response in this local test.

## Failure case and diagnosis

Stopping Node B prevented successful messaging. TCP reported connection refused when the client attempted to connect. The UDP client reported connection refused while waiting for a reply in this local environment. UDP does not guarantee delivery, and behaviour when a destination is unavailable can depend on the operating system.

## Design decision and alternative

Messages use validated JSON. TCP uses a four-byte length prefix to identify message boundaries because TCP provides a byte stream rather than separate application messages. UDP sends each message as a datagram and preserves datagram boundaries. Both transports return acknowledgements when the receiver accepts a message. Accepted messages are stored in a bounded in-memory inbox exposed through `GET /v1/messages`.

TCP provides reliable, ordered byte-stream delivery, but it needs framing to identify complete messages. UDP has less framing overhead and sends individual datagrams, but it does not guarantee delivery or ordering. UDP datagrams may also be duplicated. TCP is suitable when reliable, ordered delivery is important, while UDP demonstrates lightweight message exchange when the application can handle possible message loss. The tests demonstrate the basic behaviour of both transports but do not provide a formal performance benchmark.

A database or durable message queue could retain messages across restarts, but an in-memory inbox is sufficient for this checkpoint.

## Remaining limitations

The inbox is stored in memory and is not preserved after a node restarts. UDP can lose, duplicate or reorder messages. Acknowledgements can help the sender identify successful receipt, but they do not automatically guarantee delivery in every failure scenario. The local teaching service does not include authentication or encryption and should not be exposed to an untrusted network.

## Sources and AI-use disclosure

I consulted the Week 4 coursework guidance, the supplied project scaffold, the Go documentation, RFC 9293 for TCP and RFC 768 for UDP. ChatGPT assisted with explanations and troubleshooting. I ran the tests and live requests and checked the observed results.


# Weekly evidence record

**Student number:** 100874830

**Week:** Week 5

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week05`

**Commit hash:** `4ae1cae06ca6195d5a872563143208c4064bc429`

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




# Final integration evidence

**Student number:** 100874830

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

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
