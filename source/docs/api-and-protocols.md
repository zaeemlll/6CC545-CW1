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
