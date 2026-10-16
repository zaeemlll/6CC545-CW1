# Week 1 Evidence Record

**Student number:** 100874830
**Week:** 01
**Repository URL:** Not created yet
**Immutable tag:** week01
**Commit hash:** 668916074295da3bf61f1784a89ba14838002435
**Go version / OS:** go1.27.1 darwin/amd64; macOS Darwin 23.5.0 x86_64

## Commands executed

- `go test ./... -count=1 -timeout 20s`
- `go vet ./...`
- `go run ./cmd/request -url http://127.0.0.1:8081/healthz`
- `go run ./cmd/request -url http://127.0.0.1:8082/healthz`
- `go run ./cmd/request -url http://127.0.0.1:8099/healthz`
- `go run ./cmd/node -unknown`
- `go run ./cmd/node -id "INVALID ID"`
- `lsof -nP -iTCP:8081 -iTCP:8082`
- `go run ./cmd/node -id node-duplicate -http 127.0.0.1:8081`

## Observed results

- The full Go test suite passed across all packages.
- `go vet ./...` completed without reporting issues.
- Node A returned HTTP 200 with `{"node_id":"node-a","status":"ok"}`.
- Node B returned HTTP 200 with `{"node_id":"node-b","status":"ok"}`.
- The request to port 8099 failed with `connection refused` because no service was listening there.
- The unknown `-unknown` flag was rejected with exit status 2.
- The invalid node ID `INVALID ID` was rejected with exit status 2.
- After the nodes were stopped, `lsof` showed no processes listening on TCP ports 8081 or 8082.
- A second node using port 8081 failed with `address already in use`, as expected.

## Failure case and diagnosis

Port 8099 had no service listening, so the request failed with connection refused. The duplicate-port test failed because another node was already using port 8081. Both results were expected for these tests.

## Design decision and alternative

The node validates its configuration before starting. The health endpoint returns a simple JSON response containing the node ID and status. I used separate loopback ports to test two nodes independently. An alternative would be to allow more address configurations, but strict validation helps prevent invalid settings.

See `docs/week01-design-note.md` for the design note.

## Remaining limitations

The GitHub repository URL, student number, commit hash and Week 1 tag have not been finalised. The services do not provide TLS or authentication, so they must remain on loopback. Later weekly stages will extend the project.

## Sources and AI-use disclosure

The supplied Week 1 scaffold, assessment instructions and project tests were used as guidance. ChatGPT assisted with explanations, troubleshooting and drafting documentation. I ran the recorded commands and checked their results. I will review the implementation to make sure I understand the submitted work.
