# Implementation log

| Date | Task / commit | Decision and reason | Test run and actual result | Feedback / next step | Sources or AI assistance |
|---|---|---|---|---|---|
| 2026-10-09 | Implemented configuration validation, command-line flags, HTTP server startup and shutdown, and the `/healthz` endpoint. Implementation commit: `6689160` — Complete Week 1 configuration and health endpoint. Checkpoint: `week01`. | Separated configuration, application startup and HTTP handling so each part has a clear purpose. Added node ID and address validation to reject invalid settings. Added `/healthz` to check whether the HTTP server responds and controlled shutdown to help the server close correctly. | Ran `go test ./... -count=1 -timeout 30s`; all packages containing tests returned `ok`. Ran `go vet ./...`; no errors were reported. Manual requests to Node A and Node B returned HTTP 200. The test suite also includes configuration, unknown-flag, occupied-port and cancelled-lifecycle tests. | The tests support the configuration and server-lifecycle work. However, `/healthz` confirms that the HTTP server responds; it does not prove that other nodes or services are working. The next step was to add a REST status endpoint with JSON requests and responses. | I have consulted the official Go documentation on command line flags, HTTP servers and graceful shutdown. I have also got partial support from artificial intelligence. |
| 2026-10-11 | Week 2 — REST status resource. Implementation commit: `d4d10e2` — Implement Week 2 REST status resource. Checkpoint evidence commit: `01f69e6`; tag: `week02`. | Added a JSON GET/POST status endpoint with input validation, a 4,096-byte request-body limit and concurrency-safe state. I chose HTTP and JSON because they are readable and straightforward to test. GET reads the current status, while POST updates it. Invalid requests should be rejected without changing the stored status. | Ran `go test ./... -count=1 -timeout 30s`; all packages containing tests returned `ok`. The test listing includes `TestWeek02_REST` and `TestWeek02_ExactBodyLimit`. Targeted Week 2 tests, including the exact-body-limit test, were recorded as passing. Live HTTP checks covered GET, POST, invalid JSON and `/healthz`. The recorded invalid JSON check returned HTTP 400, and the previous status remained unchanged. | The REST endpoint provides a basic way to read and update node status. Input validation does not provide authentication or encryption. The next step was to add peer and file metadata, a peer registry and a file catalogue. | I have consulted with RFC 9110 for HTTP semantics and the OWASP REST Security Cheat Sheet for API security guidance. However, AI has provided me with partial support to review the content of this week. |
| 2026-10-12 | Week 3 — Peer registry and file catalogue. Implementation commit: `019be06` — Implement Week 3 peer registry and file catalogue. Checkpoint evidence commit: `ad11ca8`; tag: `week03`. | Added peer and file metadata validation, a concurrency-safe peer registry, a file catalogue, and the `/v1/peers` and `/v1/files` endpoints. Node A and Node B load separate seed files. I separated the registry and catalogue from the metadata models to keep responsibilities clear. I added concurrency protection because multiple requests may access the registry at the same time. | Ran `go test ./... -count=1 -timeout 30s` and `go test -race ./... -count=1 -timeout 30s`; all packages containing tests returned `ok`. Ran `go vet ./...`; no errors were reported. The test suite includes metadata validation, catalogue, registry, concurrent registry, concurrent same-peer registration and peer API tests. The same-peer concurrency test confirmed that one of 50 simultaneous registrations succeeds and the other 49 are rejected as duplicates. Live requests confirmed file listings, HTTP 201 for peer registration and HTTP 409 for a duplicate registration. | The tests support the metadata validation and concurrency behaviour. The registry stores local state, so peer information is not automatically synchronised between nodes. Stale records and failures between nodes remain limitations. The next step was to implement inter-node communication using TCP and UDP. | I have consulted the Go documentation on concurrency and the Go race detector. I have also got partial support from artificial intelligence. |
| 2026-10-19 | Week 4 — TCP/UDP inter-node messaging. Implementation commit: `71499ae` — Implement Week 4 TCP and UDP messaging. Checkpoint evidence commit: `5f83cac`; tag: `week04`. | Added validated JSON messages, length-prefixed TCP framing, UDP messaging, acknowledgements, TCP/UDP listeners and a bounded in-memory inbox exposed through `GET /v1/messages`. Updated the Node A and Node B run scripts with their TCP and UDP ports. TCP needs framing to identify message boundaries because it provides a byte stream. UDP sends datagrams but does not guarantee delivery. | Ran `go test ./... -count=1 -timeout 30s` and `go test -race ./... -count=1 -timeout 30s`; all packages containing tests returned `ok`. Ran `go vet ./...`; no errors were reported. The test suite includes TCP/UDP round trips, invalid messages, frame limits, fragmented frames, oversized TCP messages, UDP deadlines and cancelled clients. Live TCP and UDP messages were accepted in both directions, and both inbox endpoints returned HTTP 200 with received messages. With Node B stopped, the TCP client reported a connection refusal; the UDP client also reported connection refused while waiting for a response. | The prototype demonstrates both communication methods, but UDP does not guarantee delivery, and the implementation does not provide authentication or encryption. The next step was to add a defined remote-file service using Protocol Buffers and gRPC. | I have consulted RFC 9293 for TCP and RFC 768 for UDP. I have also got partial support by using artificial intelligence. |
| 2026-10-25 | Week 5 — Remote file metadata over gRPC. Implementation commit: `4ae1cae` — Implement Week 5 gRPC remote file metadata. Checkpoint: `week05`. | Added the Protocol Buffers file service, generated Go bindings, a gRPC client and server, a remote-files HTTP endpoint, bounded RPC requests and gRPC listener configuration for both node scripts. Pinned runtime and generator dependencies and recorded tool versions. I chose gRPC and Protocol Buffers to define a structured service contract for exchanging file metadata between nodes. | Ran `go test ./... -count=1 -timeout 30s` and `go test -race ./... -count=1 -timeout 30s`; all packages containing tests returned `ok`. Ran `go vet ./...`; no errors were reported. `go mod verify`, `git diff --check` and `sh scripts/check.sh` passed in the recorded Week 5 checks. The test suite includes remote-file listing, request validation, unavailable-service, cancellation and empty-catalogue tests. Live testing confirmed HTTP 201 for peer registration, HTTP 200 for remote metadata from Node B and HTTP 503 when Node B was unavailable. | The remote-file service demonstrates metadata retrieval and handles an unavailable node. However, a successful request does not prove that the connection is secure, and authentication and encryption remain limitations. The next step is to complete the required final integration checks across the nodes and confirm the complete system against the coursework acceptance criteria. | I have consulted the official Protocol Buffers documentation and gRPC guidance on deadlines. I have also got partial support from artificial intelligence. |
| 2026-10-28 | Final integration and verification | The final integration brought the weekly work together in one Go project. I checked the HTTP endpoints and the communication between Node A and Node B. I kept the existing project structure and documented the remaining limitations rather than adding features outside the CW1 scope. | Ran sh scripts/check.sh, go test -race ./... -count=1 -timeout 30s, go mod verify and git diff --check. The checks passed, the Go modules were verified, and the working diff had no whitespace errors. Live checks returned HTTP 200 for health and status, HTTP 201 for peer registration, HTTP 200 for available remote-file metadata, and HTTP 503 when the remote peer was unavailable. | The final checks supported the integration of the main components. The prototype still has limitations: it does not use TLS or authentication, and peer and file metadata are held in memory. Final submission packaging and a final check of the required files remain necessary. | ChatGPT was used for explanations and understanding. |
| 2026-11-14 | Week 6 — Event publishing and peer notification | Added a shared event bus with independent bounded subscriber queues, topic filtering, drop-newest back-pressure, coordinated cancellation and closure, and bounded duplicate detection. Integrated audit and metrics subscribers into application startup. Added an HTTP event endpoint and a forwarding worker that sends events to registered peers. I used HTTP for the process-to-process adapter because a Go channel only works inside one process. | Ran targeted event tests, race-enabled event acceptance tests, application tests, the full Go test suite, the full acceptance suite, the ordinary race-enabled full test suite and the race-enabled full acceptance suite. All passed. `go build ./...` completed without errors, `go mod verify` verified all modules, `go vet ./...` reported no issues, and `git diff --check` found no whitespace errors. Live testing registered both peers with HTTP 201. Node A accepted `live-001` with HTTP 202 and three local queue deliveries. Both Node A and Node B logged the event. Reposting the same event to Node B returned HTTP 200 with `duplicate: true` and zero additional deliveries. | The two-node demonstration confirms that events can be forwarded between separate processes and that duplicate detection prevents the demonstrated forwarding loop. Delivery remains best-effort: queues and duplicate history are in memory, full queues drop new events, and queue acceptance does not confirm processing. Physical file storage, durable reconciliation, TLS and authentication remain future work. | I consulted the supplied Week 6 workbook and integration guide, Go documentation and coursework materials. ChatGPT assisted with explanations, troubleshooting and documentation. I ran and checked the tests and live requests. |

## Week 7 — Concurrent file storage and integrity

### Task 7.1 — Storage invariant and ownership

The main storage invariant is that a successful Open returns the content and metadata for the same committed file version. An incomplete or unverified upload must not be advertised as a stored file. If a replacement fails, the previous committed file must remain unchanged.

Each node owns its storage directory. The Store manages its root directory, in-memory metadata, read/write lock and temporary-file cleanup. The caller supplies the operation context and source reader, and must close the read handle returned by Open.

### Tasks 7.2–7.4 — Validation, writing and reading

Implemented portable filename validation. Names are restricted to a defined character set and unsafe names, path separators, traversal patterns and reserved Windows device names are rejected.

Put validates the expected size and SHA256 value, checks the configured size limit, and calculates the digest while streaming the input into a temporary file inside the storage root. It checks the actual size and digest before committing the file. It also rejects additional bytes beyond the declared size and checks for cancellation. Temporary files are removed when an operation fails.

The final replacement and metadata update are protected by the Store lock. Open reads the committed content and matching metadata under the read lock, checks the content size and digest, and returns an independent read handle. Snapshot returns a detached, filename-sorted metadata list.

The application currently uses a 10 MiB maximum file size. This is an implementation choice, not a limit specified by the workbook.

### Application integration and event publishing

Connected the Store to the application. The HTTP API supports uploading files through POST /v1/files/{name}, downloading them through GET /v1/files/{name}, and listing committed metadata through GET /v1/files.

The gRPC file service receives metadata through a storage adapter, which maps the Store metadata to the existing model.FileMetadata type.

A file.added event is published only after Put successfully commits a file. Failed uploads do not publish this event. Event delivery is reported separately from storage success, so a notification failure does not undo a committed file.

The application uses a separate data directory for each node, based on its node ID. The previous seed metadata is not treated as actual stored file content.

### Task 7.5 — Concurrency and integrity tests

Added acceptance tests for portable names, successful write/read operations, metadata consistency, failed writes preserving previous content, temporary-file cleanup, cancellation, missing files, concurrent replacement of the same filename, concurrent writes to different filenames, reader failure, zero-byte files, and sorted detached snapshots.

Added HTTP tests for upload, download, successful event publishing, invalid digest rejection, file listing and rejection of unsupported listing methods. Added an application-level integration test that uploads a file through the wired application, checks the listing, downloads it and verifies the returned content and SHA256 header.

### Task 7.7 — Verification results

Tested on Go 1.27.1, darwin/amd64, with CGO enabled and Apple Clang.

The following commands completed successfully after the additional tests were added:

- go test ./... -count=1 -timeout 120s
- go test -tags acceptance ./... -count=1 -timeout 120s
- go test -race -tags acceptance ./... -count=1 -timeout 180s
- go vet ./...
- go build ./...
- go mod verify

All test packages passed, including the race-enabled acceptance tests. Static analysis and the build completed without errors, and all Go modules were verified.

### Limitations and next steps

The metadata catalogue is currently held in memory. Restart recovery is not implemented in Week 7; rebuilding and verifying the catalogue after a restart is planned for Week 8. A process-local lock does not coordinate multiple processes sharing one storage directory, so each node must have its own directory.

The temporary-file and rename approach was tested on the development macOS environment. This does not prove identical rename behaviour on Linux or other filesystems, and it does not establish crash durability or power-loss persistence. A source reader that blocks inside Read cannot necessarily be interrupted by checking its context; network adapters will need suitable read cancellation or deadlines.

The tests verify the implemented behaviour on the stated development environment. They do not establish that recovery, multi-process shared-directory access, crash durability, TLS or authentication have been implemented.

I used the supplied Week 7 workbook and integration guide. ChatGPT assisted with explanations, troubleshooting and documentation. I ran and checked the implementation and verification commands.

## Week 8 — Deployment preparation, configuration and storage recovery

### Task 8.1 — Deployment configuration

Prepared a Docker Compose configuration for three independently configured Go nodes: node-a, node-b and node-c. Each node has a unique identity, its own data volume, HTTP and gRPC listening addresses, advertised service addresses and peer seed configuration.

The nodes use a shared private bridge network and Docker service names for peer communication. HTTP ports 8081, 8082 and 8083 are published on host loopback and mapped to the internal port 8080. The internal gRPC port is 9090.

Added a multi-stage Dockerfile. Its default application target is the actual node application (`./cmd/node`), rather than the deployment scaffold. The runtime is configured to use a non-root user, a read-only root filesystem and a temporary filesystem for `/tmp`.

Added `.env.example`, `.dockerignore` and `.gitignore` to support configuration and exclude local secrets, generated binaries and unnecessary build files.

### Task 8.2 — Configuration and application lifecycle

Extended configuration to support HTTP and gRPC advertised addresses, a data directory and peer seed addresses. Environment variables and command-line flags are used to configure the application, with command-line flags taking precedence over environment variables and defaults.

Added validation for container listening addresses, advertised addresses, peer addresses and data directory configuration. Updated application startup to recover the local file catalogue before serving requests.

The `/healthz` endpoint checks local HTTP health. The `/readyz` endpoint reports local application readiness after startup and recovery. Readiness does not claim that every peer is available.

### Task 8.3 — Storage recovery and persistence

Added storage recovery to rebuild the metadata catalogue from committed files in the configured data directory. Recovery checks file names, file types, sizes and SHA-256 digests. It removes abandoned temporary files and rejects symbolic links.

Tested the actual Go node locally using a temporary data directory. Uploaded a file, stopped the node gracefully, restarted it using the same directory and retrieved the recovered file. The returned content, size and SHA-256 metadata matched the original upload.

This demonstrates local filesystem persistence across a graceful restart. I also tested Docker named-volume persistence by uploading a file to node-a, restarting its container and downloading the file again. The downloaded content matched the original file, and its SHA-256 hash remained unchanged.

The three named Docker volumes also remained after `docker compose down`. The file itself was verified across a container restart, not across a complete Compose down and up cycle.

### Task 8.4 — Remote-file and RPC validation

Added HTTP remote-files tests for successful metadata retrieval, unavailable peers and request timeouts. Added RPC service validation tests for nil requests and a missing catalogue, checking the expected gRPC status codes.

The normal and acceptance tests for the affected HTTP and RPC packages passed.

### Task 8.5 — Verification results

Tested locally on macOS 14.5, x86_64 Intel, using Go 1.27.1 darwin/amd64 with CGO enabled.

The following commands completed successfully:

- `go test -count=1 ./...`
- `go test -count=1 -tags=acceptance ./...`
- `go test -race -count=1 ./...`
- `go test -race -count=1 -tags=acceptance ./...`
- `go vet ./...`
- `go build ./...`
- `go mod verify`
- `gofmt -l $(find . -name '*.go' -type f)`
- `git diff --check`

All test packages passed. The race-enabled test suites reported no race errors. Static analysis and the build completed without errors, all modules were verified, all Go files were formatted, and the whitespace check was clean.

### Docker deployment and integration verification

After installing an Intel-compatible version of Docker Desktop, I completed the Docker deployment checks on macOS 14.5, x86_64 Intel.

The following checks passed:

- Docker Compose configuration validation.
- Docker image builds for node-a, node-b and node-c.
- Three-node container deployment.
- HTTP liveness and readiness checks for all three nodes.
- Private Docker network and service DNS reachability between all three nodes.
- Separate named volumes mounted at `/data` for each node.
- File persistence after restarting the node-a container.
- Clean shutdown using `docker compose down`, with all three named volumes remaining.

The deployment results and test details are recorded in `evidence/week08/docker-environment.md` and `evidence/week08/storage-recovery.md`.

The network tests demonstrate HTTP reachability between containers, not application-level peer registration or file replication. The persistence test demonstrates recovery after a container restart, not recovery after volume deletion or physical disk failure.

The image identifiers recorded in the evidence are local Docker image IDs, not registry digests.

The Docker deployment and Go quality checks were completed separately. The Go image version used for the lab should still be checked against the staff-approved environment before final submission.



The additional live Docker acceptance checks also passed:

- Remote gRPC metadata retrieval: a temporary gRPC client called node-b using its Compose service name and internal port 9090. The RPC returned `node_id=node-b` and an empty file list. The node-a HTTP remote-files endpoint also returned HTTP 200.
- Storage isolation: uploaded `week08-isolation-test.txt` to node-a with HTTP 201. Requesting the same filename from node-b returned HTTP 404 `file_not_found`, confirming that the nodes use separate storage.
- Peer stop and recovery: while node-b was stopped, node-a's remote-files endpoint returned HTTP 503 `peer_unavailable`. After node-b was restarted and became healthy, the same request returned HTTP 200 and identified node-b.

These checks demonstrate remote metadata communication, independent node storage and recovery of remote communication after a peer restart. They do not demonstrate automatic file replication.

I used the supplied Week 8 workbook and coursework materials. ChatGPT assisted with explanations, troubleshooting, code review and documentation. I ran the commands and checked the reported results.

The Dockerfile test targets were also built and verified successfully:

- `docker build --target test -f Dockerfile .` passed. It ran `CGO_ENABLED=1 go test -race ./... -count=1 -timeout 60s`.
- `docker build --target acceptance -f Dockerfile .` passed. It ran `CGO_ENABLED=1 go test -race -tags acceptance ./... -count=1 -timeout 120s`.

Both builds used `golang:1.27.3-bookworm`, which resolved to base image digest:

    sha256:dc0963f32053e4bc451d24d87e3892d42d01021b61f093e98eb95c0352bb84b0

This is the resolved base image digest, not a registry digest for the locally built application images. The teaching team's approval of the Go image version remains to be confirmed.

## Week 9 — Secure deployment and communication

### Task 9.1 — HTTPS and JWT authentication

Added a separate secure deployment using `compose.secure.yaml`, keeping the original Week 8 Compose configuration unchanged. The public API uses HTTPS with certificates signed by the lab certificate authority. Added JWT authentication using HS256, with checks for issuer, audience, expiry, not-before time, subject and role.

The `/healthz` endpoint remains publicly accessible. Other public API routes require a valid bearer token. Reader tokens allow read requests, while writer tokens allow write requests. Missing or invalid tokens are rejected with HTTP 401, and requests without the required role are rejected with HTTP 403.

### Task 9.2 — Mutual TLS and peer communication

Added mutual TLS for gRPC and private peer HTTPS communication. The gRPC service verifies client certificates and uses the verified peer identity when authorising remote metadata requests. It also checks JWT authentication.

Added private peer endpoints for event announcements and file transfer. These use HTTPS on port 9444 and require trusted client certificates and peer authorisation. Added a file-transfer helper that checks file names, size limits and SHA-256 values. The gRPC metadata operation remains separate from file-content transfer.

### Task 9.3 — Development credentials and deployment

Added `cmd/devcert` to generate the lab CA and node certificates, and `cmd/devtoken` to create short-lived reader and writer tokens for testing. Kept generated secrets in the ignored `secrets` directory. The CA private key remains on the host and is not mounted into the containers.

The secure Compose deployment runs node-a, node-b and node-c on a private network. It publishes only the public HTTPS ports: 8441, 8442 and 8443. The internal gRPC and private peer HTTPS ports are not published to the host. Required security mode does not fall back to plaintext communication.

### Task 9.4 — Verification results

The following checks completed successfully:

- Full Go unit tests and acceptance tests.
- Full race-enabled unit tests and race-enabled acceptance tests.
- `go build ./...`, `go vet ./...` and `go mod verify`.
- Docker Compose configuration validation and secure image builds.
- Startup and health checks for all three secure containers.
- HTTPS health request returned HTTP 200.
- A protected request without a token returned HTTP 401.
- A reader token accessed a read route successfully.
- A reader write request returned HTTP 403.
- A writer request reached the relevant handler.
- Remote gRPC metadata retrieval returned the expected node identity.
- The private peer endpoint rejected a connection without a client certificate.
- Unit tests verified mTLS file upload and download, including content and checksum validation.
- `git diff --check` completed without errors.

### Limitations and next steps

The certificates and shared JWT signing key are development credentials for the closed coursework environment, not production credentials. Peer registrations and message state remain in memory. The file catalogue is rebuilt from files in the configured data directory during startup recovery. Although the mTLS file-transfer helper is tested, complete automatic peer-to-peer file replication is not yet implemented. Durable peer membership is also still outstanding.

I used the supplied Week 9 integration guide and coursework materials. ChatGPT assisted with explanations, troubleshooting, code review and documentation. I ran the commands and checked the reported results.
