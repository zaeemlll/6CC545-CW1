# Week 7 evidence record

**Student number:** 100874830

**Week:** Week 7 — Concurrent file storage and integrity (Tasks 7.1–7.8)

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Branch:** `cw2-development`

**Immutable tag and full commit hash:** Pending Week 7 checkpoint commit and tag. The existing `week06` and `final-cw1` tags have not been changed.

**Date/time and timezone:** 15 November 2026; exact command capture time is available in the terminal history.

**OS and architecture:** macOS, darwin/amd64.

**Go version:** go1.27.1.

**Go environment:** GOOS=darwin, GOARCH=amd64, CGO_ENABLED=1, CC=clang.

**C compiler:** Apple Clang 16.0.0.

**Docker/Compose versions:** Not applicable to this Week 7 checkpoint.

## Commands actually executed

Commands were run from the `source/` directory unless otherwise stated.

- `go test -tags acceptance ./internal/storage -count=1 -timeout 60s -v`
- `go test ./internal/httpapi ./internal/rpcfiles ./internal/app -count=1 -timeout 60s -v`
- `go test ./... -count=1 -timeout 120s`
- `go test -tags acceptance ./... -count=1 -timeout 120s`
- `go test -race -tags acceptance ./... -count=1 -timeout 180s`
- `go build ./...`
- `go vet ./...`
- `go mod verify`
- `go version`
- `go env GOOS GOARCH CGO_ENABLED CC GOMOD`
- `cc --version`
- `git diff --check`

The terminal output is saved in `evidence/week07/test-results.txt`.

## Expected behaviour and requirements

The Store should validate file names and expected metadata, enforce a maximum file size, verify the actual byte count and SHA256 digest, and avoid exposing incomplete writes.

A failed replacement should preserve the previous committed file. Concurrent reads and writes should return consistent content and metadata. Snapshots should be detached and sorted. The application should publish `file.added` only after a successful storage commit.

## Observed results

- All 10 storage acceptance tests passed.
- Tests passed for filename validation, write/read operations, metadata consistency, failed writes, temporary-file cleanup, cancellation, missing files, concurrent same-name replacement, concurrent different-name writes, reader failure, zero-byte files, and sorted detached snapshots.
- HTTP tests passed for file upload, download, listing, invalid digest rejection, event publishing and unsupported listing methods.
- The gRPC storage catalogue adapter tests passed.
- The application integration test `TestWireStoresFilesThroughHTTP` passed. It uploaded a file through the wired application, checked the file listing, downloaded the content and observed the audit and metrics consumers receive the `file.added` event.
- The ordinary full Go test suite passed.
- The full acceptance test suite passed.
- The full race-enabled acceptance test suite passed.
- `go build ./...` completed without errors.
- `go vet ./...` reported no issues.
- `go mod verify` reported that all modules were verified.
- `git diff --check` reported no whitespace errors.

## Failure cases and diagnosis

The storage tests checked invalid digests, excess input, oversized claims, cancellation and source-reader failure. These cases were rejected, and the tests confirmed that a previously committed file remained available where applicable and temporary files were cleaned up.

The invalid-digest HTTP test confirmed that a rejected upload did not publish a `file.added` event.

No race-detector or compiler failure occurred in the recorded test run.

## Design decisions and interpretation

The Store uses a temporary file inside its storage root and verifies the received size and SHA256 digest before committing the replacement. A lock protects the final replacement and metadata update. Open returns content and metadata from a consistent committed version.

The application uses a 10 MiB maximum file size. This is an implementation decision, not a limit specified by the coursework.

The HTTP API provides POST and GET `/v1/files/{name}` for upload and download, and GET `/v1/files` for committed metadata. The gRPC service obtains metadata through an adapter to the existing file metadata model.

The `file.added` event is published after the storage commit. Event delivery is separate from storage success; a notification failure must not undo a committed file.

The race-enabled tests found no data races in the executed test paths. This does not prove that every possible execution is race-free. The application integration test used an HTTP test server; it was not a separate live multi-node demonstration.

## Remaining limitations and next action

The metadata catalogue is held in memory. Restart recovery is planned for Week 8 and is not claimed as a Week 7 feature.

Each node must own a separate storage directory. The process-local lock does not coordinate multiple processes sharing the same directory.

The temporary-file and rename approach was tested on macOS only. The tests do not establish identical rename behaviour on every operating system or filesystem, nor do they establish crash durability or power-loss persistence.

A generic source reader blocked inside `Read` cannot always be interrupted by checking its context. Network adapters need suitable read cancellation or deadlines.

The next action is to review the evidence and code changes, create the Week 7 checkpoint commit and immutable `week07` tag, and verify that the earlier tags remain unchanged.

## Sources and AI-use acknowledgement

I consulted the supplied Week 7 workbook, Week 7 task checklist, starter evidence template, merge instructions, final integration guide and Go documentation.

ChatGPT assisted with explanations, implementation troubleshooting and documentation. I ran the commands and reviewed the results recorded in this evidence.
