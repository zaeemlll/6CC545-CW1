# Week 2 evidence record

**Student number:** 100874830

**Week:** Week 2

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week02`

**Commit hash:** `01f69e614da67e4ecd50e9b6950154c608902f80`

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
