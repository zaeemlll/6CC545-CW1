# 6CC545 CW1 — Distributed Applications and Web Services

## Project overview

This is my cumulative CW1 project for the 6CC545 Distributed Applications and Web Services module. It is developed in weekly stages.

The project includes node configuration, HTTP health and status endpoints, peer registration, file metadata catalogues, TCP and UDP messaging, and gRPC communication using Protocol Buffers to request remote file metadata.

## Prerequisites

- Go 1.23 or later installed.
- `protoc` and the Go protobuf generators for regenerating protobuf files.
- A terminal with shell support for the supplied scripts.
- macOS, Linux, or another operating system supported by Go.

## Build and test

Run these commands from the source directory:

```bash
go build ./...
go test ./... -count=1 -timeout 30s
go vet ./...
```

Run the project's full check script with:

```bash
sh scripts/check.sh
```

## Run the nodes

Open separate terminals for the nodes.

**Node A — Terminal 1**

```bash
sh scripts/run-node-a.sh
```

**Node B — a separate terminal**

```bash
sh scripts/run-node-b.sh
```

Node A uses HTTP port `8081` and gRPC port `9091`. Node B uses HTTP port `8082` and gRPC port `9092`. Keep the services on loopback for local testing.

**Terminal 2 — testing**

Use a separate terminal to run the supplied request commands and check the endpoints.

For example:

```bash
go run ./cmd/request -url http://127.0.0.1:8081/healthz
go run ./cmd/request -url http://127.0.0.1:8082/healthz
```

A successful health request returns HTTP 200 and JSON containing the node ID and an `ok` status.

## Testing and limitations

The project has tests for configuration, HTTP endpoints, peer and file metadata, messaging, and remote gRPC requests. The recorded Go test, race test, vet, module verification, and integration checks passed.

The remote file metadata endpoint depends on the selected peer being available. If a peer is unavailable, the API can return HTTP 503. The current project is intended for local teaching and testing and does not provide TLS or authentication. File metadata queries do not transfer the actual files.

Press **Ctrl + C** in a node's terminal to stop that node.

## Further documentation

See the `docs/` directory for the weekly design notes, evidence record, and implementation log.
