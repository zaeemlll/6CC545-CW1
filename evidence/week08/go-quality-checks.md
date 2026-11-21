# Week 8 – Go Quality Checks

## Environment

Operating system: macOS 14.5
Machine architecture: x86_64 (Intel)
Go version: go1.27.1 darwin/amd64
CGO enabled: 1

## Normal tests

Command: `go test -count=1 -timeout 30s ./...`

Result: PASSED across all packages.

## Acceptance tests

Command: `go test -count=1 -timeout 60s -tags=acceptance ./...`

Result: PASSED across all packages.

## Normal race tests

Command: `go test -race -count=1 -timeout 30s ./...`

Result: PASSED across all packages. No race reports.

## Acceptance race tests

Command: `go test -race -count=1 -timeout 120s -tags=acceptance ./...`

Result: PASSED across all packages. No race reports.

## Static analysis

Command: `go vet ./...`

Result: PASSED.

## Build

Command: `go build ./...`

Result: PASSED.

## Dependency verification

Command: `go mod verify`

Result: All modules verified.

## Formatting

Command: `gofmt -l $(find . -name '*.go' -type f)`

Result: No unformatted Go files.

## Whitespace check

Command: `git diff --check`

Result: Clean.

## HTTP remote-files tests

The remote-files handler was tested against a local gRPC test server.

Tests cover successful metadata retrieval, unavailable peers, request timeouts, invalid requests and unknown peers.

Result: PASSED in normal and acceptance test runs.

## RPC service validation tests

Tests cover nil requests and a missing file catalogue.

Expected gRPC status codes:
- Nil request: `InvalidArgument`.
- Missing catalogue: `Internal`.

Result: PASSED in normal and acceptance test runs.

## Storage recovery tests

- `TestRecoverCommittedData` verifies that committed file content and SHA-256 metadata survive store recreation.
- `TestRecoverRemovesAbandonedTemporaryFiles` checks cleanup of unfinished temporary files.
- `TestRecoverRejectsSymbolicLinks` checks that recovery rejects symbolic links.
- `TestRecoverHonoursCancelledContext` checks cancellation handling.

Result: PASSED as part of the full acceptance test suite.

## Event forwarding limits

Code reviewed: `source/internal/events/bridge.go`

- Maximum forwarding attempts: 2.
- Timeout per attempt: 2 seconds.
- Failed forwarding returns an error after the attempts.
- The forwarding worker logs the failure and creates a new subscription.

Limitation: A failed peer can prevent later peers from receiving the same forwarding attempt. Failed events are not stored in a durable retry queue.

## Limitation

The Go quality checks above were run locally. They are separate from the Docker deployment tests.

Docker image builds, three-node Compose deployment, health and readiness checks, private network reachability, separate named volumes and file persistence after a node-a container restart were tested separately. Their results are recorded in `docker-environment.md` and `storage-recovery.md`.

The Docker tests do not establish application-level file replication between nodes.

## Model validation tests

Additional tests cover valid and invalid peer IDs, HTTP and gRPC addresses, peer statuses, file names and SHA-256 hashes. Boundary cases include zero-byte files, zero modification times, empty names, unsafe path characters and filenames exceeding 128 characters.

Result: PASSED in normal, acceptance and race-enabled test runs.
