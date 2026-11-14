# Week 6 evidence record

**Student number:** 100874830

**Week:** Week 6 — Event publishing and peer notification

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Branch:** `cw2-development`

**Immutable tag:** `week06` (to be created after final verification)

**Checkpoint commit:** To be recorded after checkpoint creation.

**Go version / OS:** `go1.27.1 darwin/amd64`

**Go environment:** `GOOS=darwin`, `GOARCH=amd64`, `CGO_ENABLED=1`, `CC=clang`

**C compiler:** Apple Clang 16.0.0

## Commands actually executed

- `go test ./internal/events -count=1 -timeout 30s`
- `go test -race -tags acceptance ./internal/events -count=1 -timeout 60s`
- `go test ./internal/app -count=1 -timeout 30s`
- `go test ./... -count=1 -timeout 90s`
- `go test -tags acceptance ./... -count=1 -timeout 120s`
- `go test -race -tags acceptance ./... -count=1 -timeout 180s`
- `go mod verify`
- `go build ./...`
- `go test -race ./... -count=1 -timeout 120s`
- `go vet ./...`
- `git diff --check`
- `go version`
- `go env GOOS GOARCH CGO_ENABLED CC GOMOD`
- `cc --version`
- Live HTTP requests to register Node B with Node A and Node A with Node B.
- Live POST request to publish an event to Node A.
- Live POST request to send the same event to Node B again.

## Observed results

- The targeted event tests passed.
- The race-enabled event acceptance tests passed.
- The application package tests passed.
- The full Go test suite passed.
- The full acceptance test suite passed.
- The full race-enabled acceptance test suite passed.
- `go mod verify` reported that all modules were verified.
- `go build ./...` completed without errors.
- The ordinary race-enabled full test suite passed.
- `go vet ./...` reported no issues.
- `git diff --check` reported no whitespace errors.
- Registering Node B with Node A returned HTTP 201.
- Registering Node A with Node B returned HTTP 201.
- Posting event `live-001` to Node A returned HTTP 202 with `delivered: 3`, `dropped: 0` and `duplicate: false`.
- Node A logged the event through its audit and metrics consumers.
- Node B also logged the event through its audit and metrics consumers.
- Posting the same event to Node B again returned HTTP 200 with `delivered: 0`, `dropped: 0` and `duplicate: true`.

## Failure case and diagnosis

The implementation uses bounded subscriber queues. When a subscriber queue is full, the newest attempted event is dropped for that subscriber. The publisher does not wait for a slow subscriber.

The bus records an event in its bounded duplicate history before attempting subscriber delivery. Therefore, an event that is dropped by every subscriber, or published when there are no subscribers, cannot simply be retried using the same event ID during the retained history period.

This is an intentional best-effort delivery policy, not guaranteed durable delivery.

## Design decision and alternative

I used Go channels to implement an in-process publish/subscribe event bus. Each subscriber has an independent bounded queue so that subscribers receive their own event rather than competing to consume from one shared channel.

The bus uses a drop-newest policy to prevent slow subscribers from blocking publishers. Cancellation and shutdown are coordinated by the bus to avoid unsafe channel closure.

A forwarding worker adapts the local event subscription to HTTP requests sent to registered peers. Event IDs and origins are preserved, and a bounded duplicate history prevents repeated forwarding from creating an endless loop.

An alternative would be a durable external message broker. The current implementation uses Go channels because they are suitable for the local coursework eventing checkpoint, but they do not provide persistent storage or delivery guarantees.

## Remaining limitations

- Event queues and duplicate history are held in memory.
- Delivery counts represent queue acceptance, not completed processing.
- Events can be dropped when queues are full.
- Events published without subscribers are not replayed later.
- The duplicate history retains only the most recent 4096 event keys.
- Forwarding uses HTTP without TLS or authentication.
- A failed forwarding operation is not guaranteed to be recovered.
- The current `file.added` event describes registered metadata; physical file storage and transfer are later tasks.
- The system does not yet implement durable reconciliation between peers.

## Sources and AI-use disclosure

I consulted the supplied Week 6 workbook, Week 6 task instructions, final integration guidance, Go documentation on channels and concurrency, and the existing CW1 application.

ChatGPT assisted with explanations, implementation troubleshooting and documentation preparation. I ran the listed commands and live requests and checked the observed results.
