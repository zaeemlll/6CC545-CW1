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
