# 6CC545 CW1 — Distributed Applications and Web Services

## Project overview

This repository contains my cumulative coursework project for the 6CC545 Distributed Applications and Web Services module. The project is developed in five weekly stages.

The application includes node configuration, HTTP health and status endpoints, peer registration, file metadata catalogues, TCP and UDP messaging, and gRPC communication using Protocol Buffers to request remote file metadata.

This is a coursework prototype for demonstrating distributed application concepts. It does not transfer actual file contents between nodes.

## Repository

GitHub repository: [https://github.com/zaeemlll/6CC545-CW1](https://github.com/zaeemlll/6CC545-CW1)

To clone the repository, run:

```bash
git clone https://github.com/zaeemlll/6CC545-CW1.git
cd 6CC545-CW1
```

If you already have the repository, open a terminal and move to your local repository directory instead.

## Project structure

- `source/` — Go application source code, tests, scripts, API definitions and test data.
- `repository.txt` — Repository URL.
- `test-results.txt` — Test results record.
- `source/docs/api-and-protocols.md` — HTTP API, TCP/UDP messaging and gRPC protocol documentation.
- `source/docs/evidence-record.md` — Coursework evidence record.
- `source/docs/implementation-log.md` — Weekly implementation log.
- `source/docs/week01-design-note.md` — Week 1 design note.
- `source/docs/week03-architecture.md` — Week 3 architecture document.

Check the assessment brief separately to confirm that all required submission documents and evidence files are included before submission.

## Prerequisites

For normal building and testing, you need:

- Go 1.23 or later.
- A terminal with shell support.

The application has been tested on macOS using Go 1.27.1.

The generated Protocol Buffers files are included in the project. You only need the `protoc` compiler and the Go protobuf generators if you need to regenerate those files.

## Build and test

From the repository root, enter the Go module directory:

```bash
cd source
```

Build the project:

```bash
go build ./...
```

Run the tests:

```bash
go test ./... -count=1 -timeout 30s
```

Run the tests with the race detector:

```bash
go test -race ./... -count=1 -timeout 30s
```

Check the code using `go vet`:

```bash
go vet ./...
```

Verify the Go modules:

```bash
go mod verify
```

Run the project check script:

```bash
sh scripts/check.sh
```

The check script runs `go version`, `go build ./...`, the Go test suite and `go vet ./...`. It does not replace the separate race-detector or module-verification commands.

These commands passed during development in the recorded environment. Run them again if you need to verify the current checkout.

## Running the nodes

The project provides scripts to start two local nodes. Keep each node running in its own terminal.

**Terminal 1 — Node A**

From the repository root, run:

```bash
sh source/scripts/run-node-a.sh
```

**Terminal 2 — Node B**

Open a second terminal, move to the same repository root and run:

```bash
sh source/scripts/run-node-b.sh
```

The scripts build the node executable and start the relevant node with its configuration and seed file.

Both nodes use loopback addresses for local testing.

| Service | Node A | Node B |
|---|---|---|
| HTTP | `127.0.0.1:8081` | `127.0.0.1:8082` |
| TCP | `127.0.0.1:7001` | `127.0.0.1:7002` |
| UDP | `127.0.0.1:7101` | `127.0.0.1:7102` |
| gRPC | `127.0.0.1:9091` | `127.0.0.1:9092` |

**Terminal 3 — Testing**

Open a third terminal and enter the `source/` directory. Replace the example path below with the location of your local repository:

```bash
cd /path/to/6CC545-CW1/source
```

Check Node A:

```bash
go run ./cmd/request -url http://127.0.0.1:8081/healthz
```

Check Node B:

```bash
go run ./cmd/request -url http://127.0.0.1:8082/healthz
```

A successful health request returns HTTP 200 with JSON containing the node ID and an `ok` status.

## HTTP endpoints

The application provides the following endpoints:

| Endpoint | Purpose |
|---|---|
| `/healthz` | Checks node liveness and reports the node identity. |
| `/v1/status` | Reads or updates the simulated node status. |
| `/v1/files` | Lists local file metadata. |
| `/v1/peers` | Lists or registers peers. |
| `/v1/messages` | Reads messages accepted by the node. |
| `/v1/remote-files` | Requests file metadata from a registered remote peer through gRPC. |

Check Node A's health:

```bash
curl -i http://127.0.0.1:8081/healthz
```

List Node A's local file metadata:

```bash
curl -i http://127.0.0.1:8081/v1/files
```

Read the current status:

```bash
curl -i http://127.0.0.1:8081/v1/status
```

Update the status to `degraded`:

```bash
curl -i -X POST \
  http://127.0.0.1:8081/v1/status \
  -H 'Content-Type: application/json' \
  --data '{"status":"degraded"}'
```

The status endpoint accepts `healthy` and `degraded`. Invalid requests are rejected according to the API contract documented in `source/docs/api-and-protocols.md`.

## Testing peer registration and remote metadata

Run these commands from the `source/` directory with both nodes running.

Register Node B with Node A:

```bash
curl -i -X POST \
  http://127.0.0.1:8081/v1/peers \
  -H 'Content-Type: application/json' \
  --data-binary @testdata/peer-b.json
```

A successful registration returns HTTP 201.

List registered peers:

```bash
curl -i http://127.0.0.1:8081/v1/peers
```

Request Node B's remote file metadata:

```bash
curl -i \
  'http://127.0.0.1:8081/v1/remote-files?peer_id=node-b'
```

When the peer is registered and its gRPC service is available, the request returns HTTP 200 and metadata for files such as `diagram.txt` and `notes.txt`.

Registering the same peer again returns HTTP 409 with the `duplicate_peer` error. If the remote gRPC service is unavailable, the remote metadata endpoint can return HTTP 503.

## TCP and UDP messaging

The project includes an IPC client for testing TCP and UDP communication. Run the following example from the `source/` directory while Node B is running.

Send a TCP message to Node B:

```bash
go run ./cmd/ipcclient \
  -network tcp \
  -target 127.0.0.1:7002 \
  -id node-a \
  -text "hello from node-a" \
  -request demo-tcp
```

For UDP, change `-network tcp` to `-network udp` and use a different request ID.

The client waits for an acknowledgement. The target node must be running and reachable for successful communication. UDP does not guarantee delivery, so a response is not guaranteed.

Read messages accepted by Node B:

```bash
curl -i http://127.0.0.1:8082/v1/messages
```

The inbox holds a bounded recent history in memory. Messages are not stored permanently.

## Regenerating Protocol Buffers files

This step is only needed if you change the Protocol Buffers definitions and need to regenerate the Go bindings.

The project provides a generation script at `source/scripts/generate.sh`. It requires `protoc` to be installed and available on your `PATH`. The script installs the specified Go generators, generates the bindings, runs `go mod tidy` and verifies the modules.

From the repository root, run:

```bash
cd source
sh scripts/generate.sh
```

After regeneration, review the changes and run the build and test commands again. Do not regenerate the files just to run the existing application.

## Troubleshooting

- **A port is already in use:** Check whether another copy of a node is running. Stop the existing process before restarting it.
- **A health request fails:** Confirm that the relevant node is running and that you are using its correct HTTP port.
- **Remote metadata returns HTTP 503:** Check that the target peer is registered and its gRPC service is available.
- **Peer registration returns HTTP 409:** The peer may already be registered. Check the result of `GET /v1/peers`.
- **The generation script cannot find `protoc`:** Install the required Protocol Buffers compiler and ensure it is available on your `PATH`.
- **A command cannot find a test-data file:** Check that you are running the example from the `source/` directory when the command uses a relative path such as `testdata/peer-b.json`.

## Safety and limitations

- The supplied node scripts bind services to loopback addresses for local testing. They are not configured for public network deployment.
- TLS and authentication are not implemented.
- Peer registration and file metadata catalogues are held in memory and are not persistent.
- The message inbox is held in memory and is lost when the node process stops.
- Remote file requests retrieve metadata, not actual file contents.
- Remote metadata requests depend on the selected peer and its gRPC service being available.
- The `requester_id` in the gRPC request is not proof of identity.

This project is a coursework prototype and should not be treated as a production-ready file-sharing service.

## Further documentation

- [API and Protocol Documentation](source/docs/api-and-protocols.md)
- [Evidence Record](source/docs/evidence-record.md)
- [Implementation Log](source/docs/implementation-log.md)
- [Week 1 Design Note](source/docs/week01-design-note.md)
- [Week 3 Architecture](source/docs/week03-architecture.md)
- [Test Results](test-results.txt)
