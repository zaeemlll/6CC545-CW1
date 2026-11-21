# 6CC545 CW2 — Distributed Applications and Web Services

## Prerequisites

- Go 1.23 or later.
- Git, if cloning the repository.
- A terminal with a POSIX-compatible shell.
- Docker Engine and Docker Compose v2 for the container deployment.

The Go application was developed and tested locally on macOS using Go 1.27.1. Generated Protocol Buffers and gRPC files are included in the source. Docker deployment was tested on macOS 14.5 (Intel x86_64) using Docker Desktop 4.99.0, Docker Engine 29.9.3 and Docker Compose v5.7.0. Confirm the Go image version against the staff-approved environment before final submission.

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

## CW2 Docker Compose deployment

The Compose configuration defines three independently configured nodes. Each node has its own identity and named data volume. The services communicate over the Compose network using their service names.

The three-node Docker Compose deployment was tested successfully on the development machine. Each node uses a separate named volume and communicates over the private Compose network. The Go image version should still be checked against the staff-approved environment before final submission.

Validate and build:

    docker compose config --quiet
    docker build --target test -f Dockerfile .
    docker build --target acceptance -f Dockerfile .

Start and inspect:

    docker compose up --build -d
    docker compose ps

Check readiness:

    curl http://localhost:8081/readyz
    curl http://localhost:8082/readyz
    curl http://localhost:8083/readyz

Request remote file metadata from node B through node A:

    curl "http://localhost:8081/v1/remote-files?peer_id=node-b"

Stop the deployment:

    docker compose down

This preserves named volumes. To delete the volumes and their data as well:

    docker compose down -v

| Node | Host HTTP port | Data volume |
|---|---:|---|
| node-a | 8081 | data-a |
| node-b | 8082 | data-b |
| node-c | 8083 | data-c |

The readiness endpoint checks local startup and recovery. It does not prove that every peer is reachable or that file contents have been synchronised.

## Cleanup

Stop Node A and Node B by pressing `Ctrl+C` in each node's terminal. Wait for both processes to exit.

Peer registrations and message state are held in memory and are not preserved across restarts. Committed file contents and metadata can be recovered from the configured local data directory after a restart. Docker named-volume persistence was verified by restarting node-a and retrieving the same file with matching content and SHA-256. The named volumes also remained after docker compose down.

## Known limitations

- The local two-node scripts use loopback addresses. The Docker configuration uses the private Compose network for container communication.
- TLS and application-level authentication are not implemented.
- Peer registrations and message state are held in memory. The file catalogue is rebuilt from committed files in the configured data directory during startup recovery.
- Remote file operations exchange metadata, not actual file contents.
- The prototype does not implement the full security and peer-to-peer features planned for CW2.
