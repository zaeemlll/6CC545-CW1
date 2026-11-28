# Week 10 — Final integration and live acceptance evidence

## Environment

Operating system: macOS 14.5

Machine architecture: x86_64 (Intel)

Go version: 1.27.1

Docker deployment: Docker Desktop

Compose file: `../compose.secure.yaml`

Services: node-a, node-b and node-c

Secrets directory: `../secrets-renewed`

No JWT values or private keys are included in this record.

## Automated verification

The following project-wide checks completed successfully:

    go build ./...
    go test ./... -count=1 -timeout 60s
    go test -tags acceptance ./... -count=1 -timeout 120s
    go test -race -tags acceptance ./... -count=1 -timeout 180s
    go vet ./...
    go mod verify

Observed result:

    All packages passed.
    No race warnings were reported.
    Build and static analysis completed without errors.
    all modules verified

Docker Compose configuration validation:

    docker compose -f ../compose.secure.yaml config --quiet

Result: completed without an error.

## Week 10 additional tests

### Stale versioned write protection

Test:

    go test -race ./internal/storage -run '^TestPutVersionedRejectsStaleWriteAfterNewerCommit$' -count=1 -v

Actual terminal output:

    === RUN   TestPutVersionedRejectsStaleWriteAfterNewerCommit
    --- PASS: TestPutVersionedRejectsStaleWriteAfterNewerCommit (0.42s)
    PASS
    ok      example.edu/6cc545/cw1/internal/storage 2.559s

Result: PASSED.

The test pauses an older write, commits a newer version, then releases the older write. The stale write is rejected with `ErrVersionConflict`, and the newer content and metadata remain intact.

### Storage recovery and journal acceptance tests

Normal acceptance test:

    go test -tags acceptance ./internal/storage -run 'Test(VersionedRecovery|VersionedStore|Recover)' -count=1 -v -timeout 60s

Actual terminal output:

    === RUN   TestRecoverCommittedData
    --- PASS: TestRecoverCommittedData (0.00s)
    === RUN   TestRecoverRemovesAbandonedTemporaryFiles
    --- PASS: TestRecoverRemovesAbandonedTemporaryFiles (0.00s)
    === RUN   TestRecoverRejectsSymbolicLinks
    --- PASS: TestRecoverRejectsSymbolicLinks (0.00s)
    === RUN   TestRecoverHonoursCancelledContext
    --- PASS: TestRecoverHonoursCancelledContext (0.00s)
    === RUN   TestRecoverPreservesCW2StateDirectory
    --- PASS: TestRecoverPreservesCW2StateDirectory (0.00s)
    === RUN   TestVersionedRecoveryCompletesStagedCommit
    --- PASS: TestVersionedRecoveryCompletesStagedCommit (0.25s)
    === RUN   TestVersionedRecoveryCompletesRenamedCommit
    --- PASS: TestVersionedRecoveryCompletesRenamedCommit (0.18s)
    === RUN   TestVersionedRecoveryRejectsInvalidJournalFiles
    --- PASS: TestVersionedRecoveryRejectsInvalidJournalFiles (0.23s)
    === RUN   TestVersionedRecoveryRejectsMissingStageAndDestination
    --- PASS: TestVersionedRecoveryRejectsMissingStageAndDestination (0.16s)
    === RUN   TestVersionedRecoveryRejectsCorruptedDestinationWithoutStage
    --- PASS: TestVersionedRecoveryRejectsCorruptedDestinationWithoutStage (0.14s)
    === RUN   TestVersionedRecoveryInitialisesExistingFiles
    --- PASS: TestVersionedRecoveryInitialisesExistingFiles (0.09s)
    === RUN   TestVersionedRecoveryPreservesManifestAfterRestart
    --- PASS: TestVersionedRecoveryPreservesManifestAfterRestart (0.10s)
    === RUN   TestVersionedRecoveryRejectsMissingManifest
    --- PASS: TestVersionedRecoveryRejectsMissingManifest (0.08s)
    === RUN   TestVersionedStoreBlocksReadsUntilRecovery
    --- PASS: TestVersionedStoreBlocksReadsUntilRecovery (0.08s)
    PASS
    ok      example.edu/6cc545/cw1/internal/storage 2.514s

Race-enabled acceptance test:

    go test -race -tags acceptance ./internal/storage -run 'Test(VersionedRecovery|VersionedStore|Recover)' -count=1 -v -timeout 90s

Actual terminal output:

    === RUN   TestRecoverCommittedData
    --- PASS: TestRecoverCommittedData (0.00s)
    === RUN   TestRecoverRemovesAbandonedTemporaryFiles
    --- PASS: TestRecoverRemovesAbandonedTemporaryFiles (0.00s)
    === RUN   TestRecoverRejectsSymbolicLinks
    --- PASS: TestRecoverRejectsSymbolicLinks (0.00s)
    === RUN   TestRecoverHonoursCancelledContext
    --- PASS: TestRecoverHonoursCancelledContext (0.00s)
    === RUN   TestRecoverPreservesCW2StateDirectory
    --- PASS: TestRecoverPreservesCW2StateDirectory (0.00s)
    === RUN   TestVersionedRecoveryCompletesStagedCommit
    --- PASS: TestVersionedRecoveryCompletesStagedCommit (0.22s)
    === RUN   TestVersionedRecoveryCompletesRenamedCommit
    --- PASS: TestVersionedRecoveryCompletesRenamedCommit (0.22s)
    === RUN   TestVersionedRecoveryRejectsInvalidJournalFiles
    --- PASS: TestVersionedRecoveryRejectsInvalidJournalFiles (0.12s)
    === RUN   TestVersionedRecoveryRejectsMissingStageAndDestination
    --- PASS: TestVersionedRecoveryRejectsMissingStageAndDestination (0.12s)
    === RUN   TestVersionedRecoveryRejectsCorruptedDestinationWithoutStage
    --- PASS: TestVersionedRecoveryRejectsCorruptedDestinationWithoutStage (0.17s)
    === RUN   TestVersionedRecoveryInitialisesExistingFiles
    --- PASS: TestVersionedRecoveryInitialisesExistingFiles (0.08s)
    === RUN   TestVersionedRecoveryPreservesManifestAfterRestart
    --- PASS: TestVersionedRecoveryPreservesManifestAfterRestart (0.10s)
    === RUN   TestVersionedRecoveryRejectsMissingManifest
    --- PASS: TestVersionedRecoveryRejectsMissingManifest (0.10s)
    === RUN   TestVersionedStoreBlocksReadsUntilRecovery
    --- PASS: TestVersionedStoreBlocksReadsUntilRecovery (0.10s)
    PASS
    ok      example.edu/6cc545/cw1/internal/storage 2.719s

Result: all 14 selected recovery tests passed in both runs. The race-enabled run reported no race warnings.

These tests cover committed-file recovery, abandoned temporary-file cleanup, journal completion, corrupted or missing content, manifest persistence, symbolic-link rejection, cancellation and recovery readiness. They simulate interrupted filesystem states; they do not constitute a physical power-failure test.

### Stalled TCP frame handling

Test:

    go test -race ./internal/ipc -run '^TestWeek10_ServerClosesStalledTCPFrame$' -count=1 -v

Actual terminal output:

    === RUN   TestWeek10_ServerClosesStalledTCPFrame
    --- PASS: TestWeek10_ServerClosesStalledTCPFrame (0.15s)
    PASS
    ok      example.edu/6cc545/cw1/internal/ipc 1.624s

Result: PASSED.

The test sends an incomplete TCP frame and verifies that the server closes the stalled connection without invoking the message callback.

### Runtime cancellation and shutdown

Test:

    go test -race ./internal/app -run '^TestWeek10_RunShutsDownAfterRuntimeCancellation$' -count=1 -v

Actual terminal output:

    === RUN   TestWeek10_RunShutsDownAfterRuntimeCancellation
    2026/11/28 14:42:30 node=node-test http=127.0.0.1:63130 ready
    2026/11/28 14:42:30 event metrics file.added received=0
    --- PASS: TestWeek10_RunShutsDownAfterRuntimeCancellation (0.00s)
    PASS
    ok      example.edu/6cc545/cw1/internal/app 2.250s

Result: PASSED.

The test starts the application, confirms that its health endpoint responds, cancels the runtime context and verifies that the application shuts down.

### Event bus acceptance and concurrency tests

Test file:

    internal/events/contract_test.go

Command:

    go test -race -tags acceptance ./internal/events -count=1 -v -timeout 60s

Actual terminal output:

    === RUN   TestTwoSubscribersAndTopics
    --- PASS: TestTwoSubscribersAndTopics (0.00s)
    === RUN   TestSlowSubscriberDropsNewest
    --- PASS: TestSlowSubscriberDropsNewest (0.00s)
    === RUN   TestCancelAndCloseAreIdempotent
    --- PASS: TestCancelAndCloseAreIdempotent (0.00s)
    === RUN   TestPublishCancelCloseConcurrent
    --- PASS: TestPublishCancelCloseConcurrent (0.00s)
    === RUN   TestInvalidEventRejected
    --- PASS: TestInvalidEventRejected (0.00s)
    === RUN   TestForwardDeliversThenCancels
    --- PASS: TestForwardDeliversThenCancels (0.00s)
    === RUN   TestScaffoldAllocation
    --- PASS: TestScaffoldAllocation (0.00s)
    PASS
    ok      example.edu/6cc545/cw1/internal/events  2.089s

Result: all seven tests PASSED with the race detector; no race warning was reported.

The tests cover independent subscribers, topic filtering, non-blocking handling of slow subscribers, cancellation, repeated shutdown, concurrent publish/cancel/close operations, invalid events and forwarding cancellation.


## HTTPS peer-transfer failure tests

Acceptance test file:

    internal/peerfiles/contract_test.go

The following tests exercise the HTTPS peer-download path using TLS test certificates and a versioned destination store.

### Normal acceptance test

Command:

    go test -tags acceptance ./internal/peerfiles -run '^TestWeek10_(Truncated|Cancelled|Oversized)HTTPSDownloadPreservesExistingFile$' -count=1 -v -timeout 30s

Actual terminal output:

    === RUN   TestWeek10_TruncatedHTTPSDownloadPreservesExistingFile
    --- PASS: TestWeek10_TruncatedHTTPSDownloadPreservesExistingFile (0.28s)
    === RUN   TestWeek10_CancelledHTTPSDownloadPreservesExistingFile
    --- PASS: TestWeek10_CancelledHTTPSDownloadPreservesExistingFile (0.25s)
    === RUN   TestWeek10_OversizedHTTPSDownloadPreservesExistingFile
    --- PASS: TestWeek10_OversizedHTTPSDownloadPreservesExistingFile (0.24s)
    PASS
    ok      example.edu/6cc545/cw1/internal/peerfiles       2.139s

Result: all three tests PASSED.

### Race-detector acceptance test

Command:

    go test -race -tags acceptance ./internal/peerfiles -run '^TestWeek10_(Truncated|Cancelled|Oversized)HTTPSDownloadPreservesExistingFile$' -count=1 -v -timeout 60s

Actual terminal output:

    === RUN   TestWeek10_TruncatedHTTPSDownloadPreservesExistingFile
    --- PASS: TestWeek10_TruncatedHTTPSDownloadPreservesExistingFile (0.28s)
    === RUN   TestWeek10_CancelledHTTPSDownloadPreservesExistingFile
    --- PASS: TestWeek10_CancelledHTTPSDownloadPreservesExistingFile (0.24s)
    === RUN   TestWeek10_OversizedHTTPSDownloadPreservesExistingFile
    --- PASS: TestWeek10_OversizedHTTPSDownloadPreservesExistingFile (0.23s)
    PASS
    ok      example.edu/6cc545/cw1/internal/peerfiles       2.281s

Result: all three tests PASSED with the race detector; no race warning was reported.

### Failure cases and verified outcomes

- Truncated response: the server advertises a larger response than it sends. The download fails.
- Cancelled response: the transfer is cancelled after response data starts arriving. The download stops.
- Oversized response: the server sends more bytes than the advertised file size using a chunked response. The download fails.

For all three cases, the test reopens the destination file and verifies that its previous content and metadata are unchanged.

These tests provide additional evidence for failed-transfer handling and preservation of committed files. They do not establish that every possible network failure has been tested.

## Three-node secure deployment

The existing secure containers were checked using:

    docker compose -f ../compose.secure.yaml ps

Observed status before the failure test:

    node-a: Up and healthy
    node-b: Up and healthy
    node-c: Up and healthy

All three nodes were accessible through their published HTTPS ports:

    127.0.0.1:8441 -> node-a
    127.0.0.1:8442 -> node-b
    127.0.0.1:8443 -> node-c

The internal gRPC and private peer HTTPS ports were not published to the host.

## Live TLS and authentication checks

The following health checks were performed using the lab CA certificate:

    curl --cacert ../secrets-renewed/ca.crt https://127.0.0.1:8441/healthz
    curl --cacert ../secrets-renewed/ca.crt https://127.0.0.1:8442/healthz
    curl --cacert ../secrets-renewed/ca.crt https://127.0.0.1:8443/healthz

Observed responses:

    8441 -> {"node_id":"node-a","status":"ok"}
    8442 -> {"node_id":"node-b","status":"ok"}
    8443 -> {"node_id":"node-c","status":"ok"}

Unauthenticated requests to `/v1/peers` returned:

    node-a: HTTP 401
    node-b: HTTP 401
    node-c: HTTP 401

Authenticated reader requests to `/v1/peers` returned HTTP 200 on all three nodes.

Before the failure test, each node reported the other two nodes as online.

## Live three-node failure detection test

The bootstrap node was stopped without removing its container or persistent volume:

    docker compose -f ../compose.secure.yaml stop node-a

Observed result:

    Container cw2-secure-node-a-1 Stopped

After waiting eight seconds, authenticated reader requests were sent to node-b and node-c.

Observed response from node-b, port 8442:

    node-a: offline
    node-c: online
    HTTP 200

Observed response from node-c, port 8443:

    node-a: offline
    node-b: online
    HTTP 200

This confirmed that both surviving nodes detected the unavailable node-a while continuing to recognise each other as online.

The membership configuration uses a four-second peer TTL. Expiry checks run every second, and membership announcements are sent every second.

## Live communication between surviving nodes

While node-a was stopped, an authenticated read-only request was sent from node-b to node-c:

    GET https://127.0.0.1:8442/v1/remote-files?peer_id=node-c

The request used the public HTTPS API and retrieved metadata through the private authenticated gRPC connection.

Observed response from the terminal:

    {"node_id":"node-c","files":[{"name":"cw2-replication-check.txt","size":44,"sha256":"7228fd60e4997d6910273081b3cfc961791f2fe992720815ded7f88906c382df","modified_unix":0,"version":1,"writer_id":"node-a"},{"name":"cw2-survivor-check-20261127.txt","size":43,"sha256":"7a5e91eb41e085842b0dfa4d750993b886f3c75c7a73bdf0787b4e8fd781ba35","modified_unix":0,"version":2,"writer_id":"node-b"},{"name":"cw2-version-conflict-test.txt","size":38,"sha256":"1ff463301ee01cd4ada371bb20c1585fbf516a6a7e88cd914cf8e5f452c5189b","modified_unix":0,"version":4,"writer_id":"node-a"},{"name":"writer-role-test.txt","size":32,"sha256":"53c5433f3c0d455bd63d29cdf57f2a46df00d95a46417333b878d481a6b3bf45","modified_unix":0,"version":3,"writer_id":"node-a"}]}

    HTTP 200

This demonstrated that node-b could successfully retrieve node-c's remote file metadata while node-a was unavailable.

The returned metadata included files originally written by both node-a and node-b. This request tested metadata retrieval through gRPC. The separate live file-content replication tests are recorded below.

## Node recovery test

Node-a was restarted:

    docker compose -f ../compose.secure.yaml start node-a

Observed result:

    Container cw2-secure-node-a-1 Started

The container status was checked again:

    docker compose -f ../compose.secure.yaml ps

Observed result:

    node-a: Up and healthy
    node-b: Up and healthy
    node-c: Up and healthy

After a short recovery period, authenticated reader requests to `/v1/peers` returned HTTP 200 on all three nodes.

Final peer membership:

    node-a reported node-b online and node-c online
    node-b reported node-a online and node-c online
    node-c reported node-a online and node-b online

This verified that node-a recovered and rejoined the three-node cluster.

## Live file-content replication and recovery

All three containers used separate persistent data volumes. The following tests used the authenticated public HTTPS API.

### Replication from node-a to node-b and node-c

Node-a accepted `cw2-week10-live-transfer-20261128.txt`.

Observed upload result:

    HTTP 201
    Size: 61 bytes
    SHA-256: 4f4bda79e8e183e4bb2af3549ffc4273b8aa33437e5dbbb36233da9568b8d07c
    Version: 5
    Writer: node-a

The file was then downloaded independently from node-b and node-c.

    Node-b download: HTTP 200
    Node-b size: 61 bytes
    Node-b SHA-256: 4f4bda79e8e183e4bb2af3549ffc4273b8aa33437e5dbbb36233da9568b8d07c

    Node-c download: HTTP 200
    Node-c size: 61 bytes
    Node-c SHA-256: 4f4bda79e8e183e4bb2af3549ffc4273b8aa33437e5dbbb36233da9568b8d07c

Both destination hashes matched the source hash.

### Replication while node-a was stopped

Node-a was stopped. Node-b accepted `cw2-week10-survivor-transfer-20261128.txt`.

Observed upload result:

    HTTP 201
    Size: 75 bytes
    SHA-256: 8356a9defdb0f0191232105f8c2d81b05575b2ddba900db9a753c16fad4d9aaf
    Version: 6
    Writer: node-b

While node-a remained stopped, node-c downloaded the file.

    Node-c download: HTTP 200
    Node-c size: 75 bytes
    Node-c SHA-256: 8356a9defdb0f0191232105f8c2d81b05575b2ddba900db9a753c16fad4d9aaf

The node-c hash matched the node-b source hash. This confirmed that the two surviving nodes replicated actual file content without node-a.

### Recovery of node-a

Node-a was restarted. Docker Compose reported all three containers as healthy.

An authenticated request to node-a's `/v1/peers` endpoint returned HTTP 200 and reported node-b and node-c as online.

The survivor file was then downloaded from node-a.

    Node-a recovered file: HTTP 200
    Size: 75 bytes
    SHA-256: 8356a9defdb0f0191232105f8c2d81b05575b2ddba900db9a753c16fad4d9aaf

Recovered content:

    Week 10 survivor test. Source: node-b. Node-a is stopped. Date: 2026-11-28.

The recovered hash matched the original node-b hash. This demonstrated content recovery after node-a returned.

## Result

Project build: PASSED

Full Go test suite: PASSED

Acceptance test suite: PASSED

Race-enabled acceptance test suite: PASSED

Go static analysis: PASSED

Go module verification: PASSED
Docker Compose configuration validation: PASSED

Stale versioned write protection: PASSED
Storage recovery and journal acceptance tests: PASSED (14 tests, normal and race-enabled)

Stalled TCP frame handling: PASSED

Runtime cancellation and shutdown: PASSED
Event bus acceptance and concurrency tests: PASSED
HTTPS truncated-download rejection and file preservation: PASSED
HTTPS cancelled-download rejection and file preservation: PASSED
HTTPS oversized-download rejection and file preservation: PASSED

Three-node secure deployment: PASSED

TLS health checks on all three nodes: PASSED

JWT authentication checks: PASSED

Peer failure detection: PASSED

Communication between surviving nodes: PASSED

Node restart and cluster recovery: PASSED

Live file-content replication and recovery: PASSED

## Limitations

The certificates and shared JWT signing key are development credentials for the coursework environment and are not production credentials.

Peer membership is persisted locally, but message and event state is not a fully replicated durable message history.

The live tests verified file-content replication from node-a to node-b and node-c, replication from node-b to node-c while node-a was stopped, and recovery of that file by node-a after restart. These tests covered the observed scenarios, not every possible network failure or concurrent conflict across all nodes.

The Docker failure test covered the loss and recovery of node-a while node-b and node-c remained running. It did not simulate simultaneous failures of multiple nodes.

The storage recovery tests simulated interrupted filesystem states, including staged and renamed journal operations. They did not simulate a physical power failure or prove recovery under every filesystem or hardware failure.

## Sources and AI assistance

I used the supplied CW2 coursework brief, Week 10 workbook, integration guidance and project materials.

ChatGPT assisted with explanations, troubleshooting, test planning and documentation. I ran the commands and checked the reported results.
