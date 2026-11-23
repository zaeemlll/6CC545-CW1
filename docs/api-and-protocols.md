# API and Protocol Documentation

## Week 2 — JSON REST Status Resource

### Purpose

The node provides a simulated status resource at `/v1/status`.
It supports GET to read the current value and POST to change it.
The allowed values are `healthy` and `degraded`.

The existing `/healthz` endpoint remains a separate liveness check.
It reports the node identity and `ok`. The simulated status does not
automatically stop the server or prove that other nodes are reachable.

### Request and response contract

| Request | Success or error status | Meaning |
|---|---:|---|
| GET `/healthz` | 200 | Returns node identity and liveness |
| GET `/v1/status` | 200 | Returns the current simulated status |
| POST `/v1/status` with valid JSON | 200 | Updates and returns the status |
| Unsupported method | 405 | Method not allowed; includes `Allow` |
| Missing or unsupported media type | 415 | Request must use JSON |
| Malformed JSON or unknown field | 400 | Invalid request |
| JSON null, array, extra JSON value or invalid status | 400 | Request violates the API contract |
| Request body larger than 4096 bytes | 413 | Request body exceeds the limit |

### GET example

Request:

    GET /v1/status

Example response:

    HTTP 200
    Content-Type: application/json; charset=utf-8

    {"status":"healthy"}

The returned value may be `healthy` or `degraded`, depending on the
previous successful POST request.

### POST example

Request:

    POST /v1/status
    Content-Type: application/json

    {"status":"degraded"}

Example response:

    HTTP 200
    Content-Type: application/json; charset=utf-8

    {"status":"degraded"}

The handler decodes the request into a temporary value and validates it
before changing the stored status.

### JSON body rules

- The body must contain one JSON object.
- The `status` field must be `healthy` or `degraded`.
- Unknown fields are rejected.
- Malformed JSON, null, arrays and additional JSON values are rejected.
- The maximum request body size is 4096 bytes.
- A body of exactly 4096 bytes is accepted when its content is valid.
- A body larger than 4096 bytes receives HTTP 413.
- `application/json; charset=utf-8` is accepted.
- Missing or unsupported Content-Type receives HTTP 415.

### Error response format

Errors use a JSON object with an `error` field.

For example:

    HTTP 400
    Content-Type: application/json; charset=utf-8

    {"error":"invalid_json"}

The HTTP status distinguishes successful requests from rejected requests.
The error label identifies the general reason for rejection.

### State invariant

A rejected request must not change the previously stored status.
The handler performs decoding and validation before updating the state.
The tests check that rejected requests preserve the earlier value.

### Design rationale and limitations

The resource has a clear purpose: GET reads its representation and POST
updates it. This is easier to understand and test than one endpoint that
performs unrelated operations.

HTTP 200 is suitable for a successful update because the response returns
the updated representation of an existing resource. HTTP 201 would be
more suitable for an operation that creates a new resource.

The body-size limit controls how much request data is read. Server timeouts
address how long network operations may take, so they solve a different
problem.

The decoder rejects unknown fields and extra JSON values. Go's standard
JSON decoder does not automatically reject duplicate occurrences of a
known field, which remains a limitation of this implementation. This
loopback teaching service has no authentication and is not intended for
public exposure.

### Verification

The Week 2 targeted REST tests, exact-body-limit test, full Go test suite,
and `go vet` were run during development. Live requests to Node A returned
HTTP 200 for GET and a valid POST. A malformed POST returned HTTP 400,
and a subsequent GET confirmed that the status remained `degraded`.

## Week 4 — TCP and UDP Inter-Node Messaging

### Purpose

Each node exposes TCP and UDP listeners so nodes can exchange validated
messages. Accepted incoming messages are kept in a bounded in-memory inbox
and can be read through `GET /v1/messages`.

The service is intended for local coursework demonstrations. It binds to
loopback addresses in the supplied run scripts and has no authentication.

### Message format

Messages use JSON with these fields:

| Field | Meaning |
|---|---|
| `version` | Protocol version; currently `1` |
| `request_id` | Identifier for the individual request |
| `node_id` | Identifier of the sending node |
| `text` | Message text, containing 1–512 bytes of valid UTF-8 |

Messages are rejected if the protocol version, identifiers, text, or encoded
size is invalid. Encoded messages are limited to 4096 bytes. JSON decoding
rejects unknown fields and additional JSON values.

### TCP framing

TCP is a byte stream, so each message is framed with a four-byte unsigned
big-endian length followed by that many bytes of JSON. The receiver reads
the full header and payload and rejects zero-length or oversized frames.
This allows the receiver to handle fragmented reads without assuming that
one network read corresponds to one complete message.

### UDP transport

UDP sends each encoded JSON message as a datagram. The receiver validates
the datagram and returns an acknowledgement when it can process the message.
UDP does not guarantee delivery, ordering, or retransmission. A sender may
time out or receive a network error if the receiver is unavailable.

### Acknowledgement

A successful acknowledgement uses these JSON fields:

| Field | Meaning |
|---|---|
| `version` | Protocol version |
| `request_id` | Request identifier being acknowledged |
| `receiver` | Identifier of the receiving node |
| `accepted` | Whether the message was accepted |

An acknowledgement indicates that the receiver accepted the message; it
does not imply durable storage. The inbox is held in memory and is lost when
the node process stops.

### HTTP inbox endpoint

`GET /v1/messages` returns the messages accepted by the node as a JSON array.
The inbox retains a bounded recent history of up to 20 messages.

Example request:

    GET /v1/messages

Example response:

    HTTP 200
    Content-Type: application/json; charset=utf-8

    [{"version":1,"request_id":"tcp-1","node_id":"node-a","text":"hello"}]

Only GET is supported by this endpoint. Other methods receive HTTP 405.

### Demonstration ports

| Node | HTTP | TCP | UDP |
|---|---|---|---|
| Node A | `127.0.0.1:8081` | `127.0.0.1:7001` | `127.0.0.1:7101` |
| Node B | `127.0.0.1:8082` | `127.0.0.1:7002` | `127.0.0.1:7102` |

### TCP and UDP comparison

TCP provides a reliable, ordered byte stream while the connection remains
usable, and this implementation adds explicit message framing. It is
appropriate when complete, ordered delivery matters, but connection setup
and stream handling add complexity.

UDP preserves datagram boundaries and has lower transport overhead, but
datagrams can be lost or duplicated and delivery is not guaranteed. This
implementation uses request identifiers, acknowledgements, and deadlines
to make outcomes observable; it does not implement automatic retries.

### Verification evidence

During the Week 4 demonstration, TCP and UDP messages were accepted in both
directions between Node A and Node B. The `GET /v1/messages` endpoint returned
HTTP 200 and showed the received messages. The targeted Week 4 IPC tests,
full Go test suite, race detector, and `go vet ./...` completed successfully.
When Node B was stopped, the TCP client reported connection refused; the UDP
client also reported connection refused while waiting for a response.

## Week 5 — gRPC Remote File Metadata

### Purpose

The node provides a gRPC service for requesting file metadata from a
remote node. The service uses Protocol Buffers to define the request,
response, and remote procedure call (RPC). This gives the client and
server a shared interface.

### Service contract

The service is defined in `api/files/v1/files.proto`.

| Item | Definition |
|---|---|
| Service | `FileService` |
| RPC method | `ListFiles` |
| Request | `ListFilesRequest` |
| Response | `ListFilesResponse` |
| Request field | `requester_id` — string |
| Response fields | `files` — repeated `FileMetadata`; `node_id` — string |

Each `FileMetadata` message contains these fields:

| Field | Type | Meaning |
|---|---|---|
| `name` | string | File name |
| `size` | int64 | File size |
| `sha256` | string | SHA-256 checksum |
| `modified_unix` | int64 | Modification time as a Unix timestamp |

### Request and response

The client calls `ListFiles` and sends a `ListFilesRequest` containing
the requester ID. The server responds with a list of file metadata and
the identity of the responding node.

The metadata describes files; it does not transfer their contents.
Protocol Buffers define the message structure, while gRPC provides the
remote call between the client and server.

### Error handling and limitations

A remote call may fail if the server is unavailable or the request is
cancelled. The client must handle these errors rather than assuming
that every call will succeed. An empty file list is also a possible
response and should be handled without treating it as a failure.

The `requester_id` field is an unverified claim, not proof of identity.
This coursework service does not provide authentication through that
field. It is intended for a local teaching demonstration, not
unprotected public deployment.

### Verification

The Week 5 tests cover successful remote listing, unavailable servers,
cancelled requests, empty results, and request validation. The full Go
test suite, race detector, and `go vet ./...` also completed successfully
during development.

For submission evidence, include the actual server and client commands
and their observed output. Record any failure case and its observed
result; do not use example output as if it were a real test transcript.

## Week 6 — Event publishing and peer notification

### Purpose

Week 6 adds an event bus to each node. It allows application components
to publish file metadata events without depending directly on individual
subscribers.

The application creates one shared event bus during startup. Separate
subscribers receive their own copies through independent bounded queues.
The implementation includes audit logging, a metrics consumer and a
worker that forwards events to registered peers.

The event describes metadata only. At this stage, the project does not
store or transfer physical file contents. The `file.added` event therefore
represents registered file metadata, not a completed physical file upload.

### Event contract

An event is represented by the following JSON fields:

| Field | Meaning |
|---|---|
| `id` | Event identifier used for duplicate detection |
| `kind` | Event topic, such as `file.added` |
| `origin` | Identifier of the node that created the event |
| `name` | File name associated with the event |

Example:

    {
      "id": "live-001",
      "kind": "file.added",
      "origin": "node-a",
      "name": "integration-demo.txt"
    }

The event contains metadata and does not include file contents.

### Subscription and delivery

`Subscribe(topic)` creates an independent subscription for a topic.
It returns a receive-only channel and an idempotent cancellation function.
The bus owns the subscriber channels and coordinates their closure.

`Publish(event)` validates the event and attempts a non-blocking send
to each matching subscriber. A slow subscriber cannot block the publisher.

Each subscriber has a bounded queue. If its queue is full, the newest
event for that subscriber is dropped. The `Delivery` result reports the
number of accepted queue deliveries and dropped deliveries.

A reported delivery means that an event was accepted into a subscriber
queue. It does not confirm that the subscriber has finished processing it.

### Back-pressure and duplicate handling

The queue capacity used by the application is 64 events per subscriber.
The bus uses a drop-newest policy when a queue is full. It does not create
a goroutine for each attempted delivery.

The bus remembers a bounded recent history of up to 4096 event identifiers.
The duplicate key combines the origin and event ID. A duplicate event
within this retained history is not delivered again.

The history is held in memory. An event may be lost if it is published
when there are no subscribers or if all relevant subscriber queues are
full. Such an event is still recorded in the recent duplicate history.
After an old entry is removed from that bounded history, the same event
could be accepted again.

This is best-effort, at-most-once delivery within the retained duplicate
window. It is not a durable message queue and does not guarantee that
every subscriber processes every event.

### Peer forwarding

The forwarding worker subscribes to `file.added` events and sends them
to registered peers using their HTTP `/v1/events` endpoint. This uses
network communication between separate node processes; a Go channel
alone cannot communicate between processes.

Each forwarding attempt has a two-second timeout and the worker makes
up to two attempts for a send. If forwarding continues to fail, the
worker reports the failure and restarts its subscription for later events.
The forwarding adapter stops at the first peer error, so later peers in the snapshot may not receive that event. Because the worker retries the forwarding operation, peers that already accepted it may receive the same event again; receiver-side duplicate detection suppresses it while the ID remains in the retained history. A failed event is not guaranteed to be delivered later.

The original event ID and origin are preserved. A receiving node publishes
the event locally, but its duplicate detection prevents the same event
from circulating indefinitely between the nodes while its identifier
remains in the retained history.

The current peer forwarding endpoint uses HTTP on loopback addresses.
It does not provide TLS or authentication. Secure peer communication is
a later CW2 task.

### HTTP event endpoint

`POST /v1/events` accepts one JSON event.

| Request or condition | HTTP status | Meaning |
|---|---:|---|
| Valid new event | 202 | Event accepted by the local bus |
| Previously seen event | 200 | Duplicate recognised |
| Invalid event or JSON | 400 | Request rejected |
| Unsupported media type | 415 | JSON content type required |
| Unsupported method | 405 | Only POST is supported |
| Closed event bus | 503 | Event service unavailable |

Example request:

    POST /v1/events
    Content-Type: application/json

    {
      "id": "live-001",
      "kind": "file.added",
      "origin": "node-a",
      "name": "integration-demo.txt"
    }

A new event returns HTTP 202 with delivery counts. A duplicate returns
HTTP 200 with `duplicate` set to true.

### Verification

The event tests cover independent subscribers, topic filtering, a full
subscriber queue, invalid events, cancellation, closure, concurrent
publishing and forwarding behaviour.

The application was also tested using two separate running node processes.
Both peers were registered successfully with HTTP 201. Posting a new event
to Node A returned HTTP 202 with three local deliveries and zero drops.
Node B logged the event through its audit and metrics consumers. Posting
the same event to Node B again returned HTTP 200 with `duplicate: true`
and zero additional deliveries.

The full Go test suite, acceptance tests, race-enabled acceptance tests
and `go vet ./...` completed successfully during this development stage.

### Limitations and future work

The event bus is in-memory and does not survive a process restart.
Queue acceptance does not confirm successful processing. Events may be
dropped under back-pressure or when a peer is unavailable. The bounded
duplicate history also means that very old duplicate events may eventually
be accepted again.

There is no durable replay or automatic repair of missed notifications.
Later reconciliation work is intended to address differences between
nodes. Physical file storage and transfer are also not implemented at
this stage. The current peer HTTP communication is unauthenticated and
must be upgraded as part of the later security work.

## Week 8 — Docker Compose deployment configuration

### Purpose

Week 8 prepares the application to run as three independently configured
nodes using Docker Compose. Each node uses the same application image but
has its own identity, network addresses, peer seeds and persistent data
volume.

The intended services are `node-a`, `node-b` and `node-c`. They communicate
over a dedicated Docker bridge network.

### Node configuration

| Setting | Node A | Node B | Node C |
|---|---|---|---|
| Node identity | `node-a` | `node-b` | `node-c` |
| HTTP host port | 8081 | 8082 | 8083 |
| HTTP container port | 8080 | 8080 | 8080 |
| gRPC container port | 9090 | 9090 | 9090 |
| Data directory | `/data` | `/data` | `/data` |
| Docker volume | `data-a` | `data-b` | `data-c` |

Each node listens on `0.0.0.0:8080` for HTTP and
`0.0.0.0:9090` for gRPC inside its container. The host HTTP ports are
mapped to the same internal HTTP port on each service.

### Listen and advertised addresses

A listen address tells a service where to accept incoming connections.
An advertised address tells other nodes which address to use when
connecting to it.

The Compose configuration advertises the Docker service names:

| Node | HTTP advertised address | gRPC advertised address |
|---|---|---|
| Node A | `http://node-a:8080` | `node-a:9090` |
| Node B | `http://node-b:8080` | `node-b:9090` |
| Node C | `http://node-c:8080` | `node-c:9090` |

Docker's internal DNS resolves these service names on the Compose
network. DNS provides address resolution; it does not itself register
peers, authenticate them or synchronise files.

### Peer seeds and storage

Each node is configured with the other two nodes as its peer seeds.
For example, Node A uses `node-b:9090,node-c:9090`.

Each service has a separate named volume mounted at `/data`. This keeps
the nodes' storage separate instead of making them use one shared
directory. The application uses `DATA_DIR` to select its local storage
directory.

The Compose configuration is intended to preserve these named volumes
when containers are stopped and recreated. The command
`docker compose down -v` removes the Compose volumes and their stored
data.

### Configuration precedence

The application supports configuration through command-line flags,
environment variables and built-in defaults. Command-line flags take
precedence over environment variables, which take precedence over
defaults.

The Week 8 Compose configuration uses these environment variables:

- `NODE_ID` — the identity of the node.
- `HTTP_ADDR` — the HTTP listen address.
- `GRPC_ADDR` — the gRPC listen address.
- `HTTP_ADVERTISE` — the HTTP address advertised to peers.
- `GRPC_ADVERTISE` — the gRPC address advertised to peers.
- `DATA_DIR` — the local data directory.
- `PEER_SEEDS` — the other nodes' gRPC addresses.

Configuration is validated during startup. The supplied `.env.example`
contains the configurable image and application target settings; it
does not contain credentials or private keys.

### Health and readiness

The application provides two separate HTTP endpoints:

| Endpoint | Purpose |
|---|---|
| `GET /healthz` | Reports the node's local liveness and identity |
| `GET /readyz` | Reports whether the local application has completed startup and storage recovery |

Readiness is a local check. A successful response does not prove that
other nodes are reachable, that peer communication is working, or that
files have been synchronised.

The Compose health check uses `/readyz` on the local container address.
It does not use the health check as proof of the whole distributed
system's health.

### Deployment verification and limitations

The Week 8 deployment was verified on macOS 14.5 (Intel x86_64) using
Docker Desktop. The three services built and started successfully. Health
checks passed, cross-container gRPC metadata retrieval was tested, and
separate named volumes were checked for storage isolation and persistence
after a container restart. The detailed results are recorded in the
Week 8 evidence files.

The original Week 8 Compose configuration uses plaintext communication.
The separate Week 9 secure Compose configuration adds TLS and peer
authentication without changing the original Week 8 deployment.

## Week 9 — Secure deployment and communication

### Purpose

Week 9 adds a separate secure deployment for the three-node application. It protects public API requests using HTTPS and JWT authentication, and uses mutual TLS (mTLS) for communication between trusted nodes. The original Week 8 deployment remains available.

### Secure deployment and ports

The secure deployment is defined in `compose.secure.yaml`. It runs `node-a`, `node-b` and `node-c` on a private Compose network.

| Service | Internal port | Purpose |
|---|---:|---|
| HTTPS API | 8443 | Public API, protected by TLS and JWT |
| gRPC | 9443 | Remote metadata service, protected by mTLS |
| Private peer HTTPS | 9444 | Peer announcements and file transfer, protected by mTLS |

The host publishes only the HTTPS API ports: 8441 for node-a, 8442 for node-b and 8443 for node-c. The gRPC and private peer HTTPS ports are not published to the host.

### HTTPS and JWT authentication

The public API uses server certificates signed by the lab certificate authority. Clients verify the server certificate using the CA certificate. Required security mode does not fall back to plaintext HTTP.

`GET /healthz` remains publicly accessible for liveness checks. Other public API routes require a valid bearer JWT. Tokens use HS256 and are checked for issuer, audience, expiry, not-before time, subject and role.

- `reader` permits read-only API requests.
- `writer` permits write requests, including event publishing.
- Missing or invalid tokens receive HTTP 401.
- A valid token without the required role receives HTTP 403.

For example, the health endpoint can be checked using:

    curl --cacert secrets/ca.crt https://localhost:8441/healthz

Protected routes also require an appropriate bearer token. Never include tokens or private keys in evidence or commit them to the repository.

### Mutual TLS between nodes

The gRPC service uses TLS client-certificate verification. The server checks the connecting peer certificate and uses its verified identity when authorising remote file metadata requests. gRPC requests also use the JWT authentication interceptor implemented in this project.

Private peer communication uses HTTPS on port 9444. Peer seeds and advertised peer HTTP addresses use HTTPS URLs on this port, rather than the public API port or the gRPC port.

The private listener provides:

- `/v1/peer/announce` for peer event announcements.
- `/v1/peer/files` for file-content transfer.

Both endpoints require a trusted client certificate and peer authorisation checks.

The peer file-transfer helper uses mTLS and checks file names, size limits and SHA-256 values. The gRPC `ListFiles` operation continues to return metadata; it does not itself transfer file contents.

### Development credentials and configuration

The `cmd/devcert` helper generates the lab certificate authority and node certificates. The `cmd/devtoken` helper creates short-lived reader or writer tokens for testing.

The CA private key remains on the host and should never be mounted into containers. Node private keys and the JWT signing key are kept in the ignored `secrets` directory and mounted read-only where required.

The secure deployment uses `SECURITY_MODE=required` and the TLS, CA and JWT configuration settings. Development certificates and the shared lab JWT signing key are suitable only for the closed coursework environment. They are not production credentials.

### Verification and limitations

The Go unit, acceptance and race-enabled acceptance test suites passed. Docker Compose configuration validation and image builds completed successfully, and all three secure containers became healthy.

Live checks confirmed that:

- `/healthz` returned HTTP 200.
- A protected route rejected a missing token with HTTP 401.
- A reader could access a read route.
- A reader could not perform a write request and received HTTP 403.
- A writer request reached the relevant handler.
- The private peer endpoint rejected a connection without a client certificate.
- The gRPC metadata endpoint returned the expected remote node identity.

Unit tests cover mTLS file upload and download, including content and checksum validation.

The certificates and shared JWT key are for development only. Peer registrations and message state remain in memory. The file catalogue is rebuilt from files in the configured data directory during startup recovery. The project does not yet provide every planned distributed-system feature, such as durable peer membership and complete automatic peer-to-peer file replication.
