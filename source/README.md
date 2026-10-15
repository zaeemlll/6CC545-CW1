# 6CC545 CW1 — Distributed Applications and Web Services

## Project overview

This is my cumulative CW1 project for the 6CC545 Distributed Applications and Web Services module. The project is being developed in weekly stages.

The Week 1 implementation validates node configuration and provides an HTTP health endpoint that returns JSON. Two local nodes can run on separate ports so their responses can be checked independently.

## Prerequisites

- Go installed. The project declares a Go 1.23 language compatibility baseline.
- A terminal with Bash for the supplied shell scripts.
- macOS, Linux, or another operating system supported by Go.

## Build and test

Run these commands from the source directory:

```bash
cd ~/Downloads/6CC545-CW1/source
go build ./...
go test ./... -count=1 -timeout 20s
go vet ./...
```

The recorded Week 1 test suite passed, and `go vet` completed without reporting issues.

## Run the nodes

Open separate terminals.

**Node A — Terminal 1**

```bash
cd ~/Downloads/6CC545-CW1/source
sh scripts/run-node-a.sh
```

**Node B — Terminal 3**

```bash
cd ~/Downloads/6CC545-CW1/source
sh scripts/run-node-b.sh
```

Node A listens on `127.0.0.1:8081`. Node B listens on `127.0.0.1:8082`. Keep both services on loopback.

## Check the health endpoint

In Terminal 2, run:

```bash
cd ~/Downloads/6CC545-CW1/source
go run ./cmd/request -url http://127.0.0.1:8081/healthz
go run ./cmd/request -url http://127.0.0.1:8082/healthz
```

A successful request returns HTTP 200 and JSON containing the node ID and an `ok` status.

To stop a node, press **Ctrl + C** in its terminal.

## Testing and limitations

Week 1 checks cover configuration validation, health responses, invalid input, an unavailable port, and a duplicate-port failure.

These services are for local teaching and testing. They do not provide TLS or authentication. Do not expose them to other networks. Later weekly stages will extend this project.
