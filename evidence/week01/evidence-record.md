# Week 1 evidence record

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
