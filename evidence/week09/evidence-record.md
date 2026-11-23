# Week 9 — Secure deployment and communication evidence

## Environment

Operating system: macOS 14.5

Machine architecture: x86_64 (Intel)

Go version: 1.27.1

Docker deployment: Docker Desktop

## Secure deployment configuration

Compose file:

    compose.secure.yaml

Services:

    node-a
    node-b
    node-c

The secure deployment uses a private Docker network. Only the public HTTPS API ports are published to the host.

## Secure container status

Command:

    docker compose -f compose.secure.yaml ps

Observed result:

    node-a: Up and healthy, 127.0.0.1:8441 -> 8443
    node-b: Up and healthy, 127.0.0.1:8442 -> 8443
    node-c: Up and healthy, 127.0.0.1:8443 -> 8443

All three secure containers were running and healthy when this status was checked.

The internal gRPC port 9443 and private peer HTTPS port 9444 are not published to the host.

## TLS and public API verification

The public API uses HTTPS with certificates signed by the lab certificate authority. The client verifies the server certificate using the CA certificate.

Health check:

    curl --cacert secrets/ca.crt -i https://localhost:8441/healthz

Result:

    HTTP 200 OK

The health endpoint remains accessible without a JWT for liveness checks.

A protected request without a bearer token returned:

    HTTP 401 Unauthorized

A request using a valid reader token returned:

    HTTP 200 OK

A reader token attempting a write request returned:

    HTTP 403 Forbidden

A writer-token request reached the relevant API handler. The duplicate peer registration test returned HTTP 409, confirming that the request passed the authorisation check and reached duplicate validation.

Tokens and private keys are intentionally excluded from this evidence.

## JWT authentication

JWTs use HS256 and are validated for issuer, audience, expiry, not-before time, subject and role.

The configured roles are:

    reader — read-only API requests
    writer — write API requests

Missing or invalid tokens are rejected with HTTP 401. A valid token without the required role is rejected with HTTP 403.

## Mutual TLS verification

The gRPC service uses TLS client-certificate verification and checks the verified peer identity when authorising remote metadata requests. The gRPC service also applies JWT authentication.

The private peer HTTPS listener uses mutual TLS on port 9444.

A live request to the private peer endpoint without a client certificate was rejected during the TLS handshake:

    certificate required

The gRPC metadata endpoint was also tested and returned the expected remote node identity.

## Peer file-transfer tests

The peer file-transfer helper uses mutual TLS. Unit tests verified upload and download, including file content, size and SHA-256 validation.

The gRPC ListFiles operation returns metadata; it does not transfer file contents.

## Automated verification

The following checks completed successfully:

    go test ./... -count=1
    go test -tags=acceptance ./... -count=1
    go test -race ./... -count=1
    go test -race -tags=acceptance ./... -count=1
    go build ./...
    go vet ./...
    go mod verify

Result:

    All test packages passed.
    No race errors were reported.
    Build and static analysis completed without errors.
    All Go modules were verified.

Docker Compose configuration validation and secure image builds also completed successfully.

The Dockerfile test and acceptance targets were built successfully. They ran the race-enabled Go test suites in the Docker build environment.

## Result

HTTPS health check: PASSED

JWT authentication: PASSED

Reader and writer role checks: PASSED

gRPC mutual TLS and peer identity checks: PASSED

Private peer HTTPS client-certificate rejection: PASSED

mTLS file-transfer unit tests: PASSED

Three-node secure deployment: PASSED

Go unit, acceptance and race-enabled test suites: PASSED

Docker configuration validation and image builds: PASSED

## Limitations

The certificates and shared JWT signing key are development credentials for the closed coursework environment, not production credentials.

Peer registrations and message state remain in memory. The file catalogue is rebuilt from files in the configured data directory during startup recovery.

The file-transfer helper is tested, but complete automatic peer-to-peer file replication is not implemented. Durable peer membership is also outstanding.

The successful unit tests for file transfer do not establish that a complete live file-replication workflow between the deployed nodes has been tested.

## Sources and AI assistance

I used the supplied Week 9 coursework integration guide and related project materials. ChatGPT assisted with explanations, troubleshooting, code review and documentation. I ran the commands and checked the reported results.
