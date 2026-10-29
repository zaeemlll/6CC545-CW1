# 6CC545 CW1 — Distributed Applications and Web Services

## Prerequisites

- Go 1.23 or later.
- Git, if cloning the repository.
- A terminal with a POSIX-compatible shell.

The project was developed and tested on macOS using Go 1.27.1. Generated Protocol Buffers and gRPC files are included in the source.

Clone the repository:

```bash
git clone https://github.com/zaeemlll/6CC545-CW1.git
cd 6CC545-CW1
```

## Build and test

Run these commands from the repository root:

```bash
cd source
go build ./...
go test ./... -count=1 -timeout 30s
go test -race ./... -count=1 -timeout 30s
go vet ./...
go mod verify
```

Expected output: the build, tests, race detector and static analysis complete successfully. Dependency verification reports that all downloaded modules are verified.

## Start the two nodes

Run each command in a separate terminal from the repository root.

**Terminal 1 — Node A**

```bash
sh source/scripts/run-node-a.sh
```

**Terminal 2 — Node B**

```bash
sh source/scripts/run-node-b.sh
```

Keep both terminals open while testing.

## Ports

| Service | Node A | Node B |
|---|---|---|
| HTTP/REST | `127.0.0.1:8081` | `127.0.0.1:8082` |
| TCP | `127.0.0.1:7001` | `127.0.0.1:7002` |
| UDP | `127.0.0.1:7101` | `127.0.0.1:7102` |
| gRPC | `127.0.0.1:9091` | `127.0.0.1:9092` |

## Test the interfaces

Run the following commands in a third terminal from the repository root.

**Check Node A health**

```bash
curl -i --max-time 5 http://127.0.0.1:8081/healthz
```

**Check Node B health**

```bash
curl -i --max-time 5 http://127.0.0.1:8082/healthz
```

Expected output: HTTP `200` and a JSON response containing the node identity and `ok` status.

**Read Node A status**

```bash
curl -i --max-time 5 http://127.0.0.1:8081/v1/status
```

**Update Node A status**

```bash
curl -i --max-time 5 \
  -X POST http://127.0.0.1:8081/v1/status \
  -H 'Content-Type: application/json' \
  -d '{"status":"degraded"}'
```

Expected output: a successful response for a valid status value. Invalid JSON, unsupported content types, invalid values and oversized requests should be rejected without corrupting state.

**List local file metadata**

```bash
curl -i --max-time 5 http://127.0.0.1:8081/v1/files
```

**Register Node B with Node A**

```bash
curl -i --max-time 5 \
  -X POST http://127.0.0.1:8081/v1/peers \
  -H 'Content-Type: application/json' \
  --data-binary @source/testdata/peer-b.json
```

**List registered peers**

```bash
curl -i --max-time 5 http://127.0.0.1:8081/v1/peers
```

**Request remote file metadata**

```bash
curl -i --max-time 5 \
  'http://127.0.0.1:8081/v1/remote-files?peer_id=node-b'
```

Expected output: when the peer is registered and its gRPC service is available, the response contains remote file metadata. An unavailable remote service is handled with HTTP `503`; duplicate peer registration is handled with HTTP `409`.

The other interfaces are TCP, UDP and gRPC. Use the project's IPC client and the tests to exercise these interfaces. To see the IPC client's supported options, run:

```bash
cd source
go run ./cmd/ipcclient -h
```

Refer to `docs/api-and-protocols.md` for the supported request formats and protocol details.

## Cleanup

Stop Node A and Node B by pressing `Ctrl+C` in each node's terminal. Wait for both processes to exit.

The project uses in-memory state for peer registrations, catalogues and messages. This state is not preserved across restarts.

## Known limitations

- The services are configured for local loopback testing.
- TLS and application-level authentication are not implemented.
- Peer registrations, catalogues and message state are held in memory.
- Remote file operations exchange metadata, not actual file contents.
- The prototype does not implement the full security and peer-to-peer features planned for CW2.