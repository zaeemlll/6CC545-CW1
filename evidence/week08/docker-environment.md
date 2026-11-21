# Week 8 – Docker Environment Evidence

## Environment

Operating system: macOS 14.5
Machine architecture: x86_64 (Intel)

## Docker installation

Application path:

    /Applications/Docker.app

Docker Desktop version:

    4.99.0

Docker client:

    29.9.3
    darwin/amd64

Docker Engine:

    29.9.3
    linux/amd64

Docker Compose:

    v5.7.0

The Docker CLI and Docker Engine were both available and working.

## Docker Compose configuration

Command:

    docker compose config --quiet

Result:

    Docker Compose configuration is valid

Command:

    docker compose config --services

Result:

    node-a
    node-b
    node-c

## Image build

Command:

    docker compose build

Result:

    Successfully built the three node images.

Local Docker image IDs:

    node-a: sha256:b8b8274968cb4a2d16c10fbdae706e55f5eca66d662928e4093fe44e8a86dad8
    node-b: sha256:53e5262e755489494100c4258d753e236d1aa7fd80f1b57a7063181ce20a31d9
    node-c: sha256:17ce391c2c7db8d2881c3d900e0bd2fbfa220f8ce1b5eabf68b9ead61a3362df

These are local Docker image IDs, not registry digests.

## Three-node deployment

Command:

    docker compose up -d

Result:

    Three containers started successfully.

Command:

    docker compose ps

Result:

    node-a: Up and healthy, 127.0.0.1:8081 -> 8080
    node-b: Up and healthy, 127.0.0.1:8082 -> 8080
    node-c: Up and healthy, 127.0.0.1:8083 -> 8080

## Health and readiness checks

The following endpoints returned HTTP 200:

    http://127.0.0.1:8081/healthz
    http://127.0.0.1:8082/healthz
    http://127.0.0.1:8083/healthz

The responses identified node-a, node-b and node-c respectively.

The following readiness endpoints also returned HTTP 200:

    http://127.0.0.1:8081/readyz
    http://127.0.0.1:8082/readyz
    http://127.0.0.1:8083/readyz

All three nodes reported status ready.

## Private network and peer reachability

The peer lists showed the other two configured nodes for each node.

The Docker probe successfully checked HTTP reachability in all six directions:

    node-a -> node-b
    node-a -> node-c
    node-b -> node-a
    node-b -> node-c
    node-c -> node-a
    node-c -> node-b

This confirms private Docker DNS and HTTP reachability. It does not by itself prove application-level peer registration or file replication.

## Separate named volumes

Docker inspection confirmed:

    node-a: cw2-lab_data-a -> /data
    node-b: cw2-lab_data-b -> /data
    node-c: cw2-lab_data-c -> /data

Each node has a separate named volume mounted at /data.

## Docker volume persistence test

A 35-byte file named persistence-test.txt was uploaded to node-a.

Upload result:

    HTTP 201 Created

SHA-256 before restart:

    8246babb2456aa456df9ec9f5024cc7ca3562c86b1f9c94fabced72671f0f653

The downloaded file matched the original before restarting node-a.

Command:

    docker compose restart node-a

Result:

    node-a restarted and returned to healthy status.
    node-b and node-c remained healthy.

The file was downloaded again after the restart. The byte comparison succeeded and its SHA-256 remained:

    8246babb2456aa456df9ec9f5024cc7ca3562c86b1f9c94fabced72671f0f653

The file contents and size also matched the original.

This confirms that node-a recovered its stored file from its named Docker volume after a container restart.

## Result

Docker Compose configuration validation: PASSED

Docker image build: PASSED

Three-node deployment: PASSED

Health and readiness checks: PASSED

Private Docker DNS and HTTP reachability: PASSED

Separate named volumes: CONFIRMED

File persistence after node-a container restart: PASSED

## Evidence limitations

The persistence test verifies recovery after a container restart. It does not prove persistence after deleting Docker volumes.

The peer reachability checks verify network connectivity, not application-level file replication.

The recorded image IDs are local image IDs, not registry digests.

## Clean shutdown and volume preservation

Command:

    docker compose down

Result:

    All three containers were removed successfully.
    The Docker network was removed successfully.

Command:

    docker volume ls

Result:

    cw2-lab_data-a
    cw2-lab_data-b
    cw2-lab_data-c

All three named volumes remained after the containers and network
were removed. The command did not use the -v option, so the volumes
and their stored data were preserved.

## Exact image references and live node identities

Command:

    docker image inspect --format='{{.RepoTags}} | ID={{.Id}} | RepoDigests={{json .RepoDigests}}' cw2-lab-node-a:latest cw2-lab-node-b:latest cw2-lab-node-c:latest

Docker inspection output:

    [cw2-lab-node-a:latest] | ID=sha256:b8b8274968cb4a2d16c10fbdae706e55f5eca66d662928e4093fe44e8a86dad8 | RepoDigests=["cw2-lab-node-a@sha256:b8b8274968cb4a2d16c10fbdae706e55f5eca66d662928e4093fe44e8a86dad8"]
    [cw2-lab-node-b:latest] | ID=sha256:53e5262e755489494100c4258d753e236d1aa7fd80f1b57a7063181ce20a31d9 | RepoDigests=["cw2-lab-node-b@sha256:53e5262e755489494100c4258d753e236d1aa7fd80f1b57a7063181ce20a31d9"]
    [cw2-lab-node-c:latest] | ID=sha256:17ce391c2c7db8d2881c3d900e0bd2fbfa220f8ce1b5eabf68b9ead61a3362df | RepoDigests=["cw2-lab-node-c@sha256:17ce391c2c7db8d2881c3d900e0bd2fbfa220f8ce1b5eabf68b9ead61a3362df"]

These are the exact local image references reported by Docker. The images were not pushed to a remote registry.

Command:

    for port in 8081 8082 8083; do echo "----- PORT $port -----"; curl -fsS "http://127.0.0.1:$port/healthz"; echo; done

Live health responses:

    Port 8081: {"node_id":"node-a","status":"ok"}
    Port 8082: {"node_id":"node-b","status":"ok"}
    Port 8083: {"node_id":"node-c","status":"ok"}

All three live node identities were confirmed.

## Remote gRPC metadata request

A temporary gRPC client was run in a container on the private Compose
network to call node-b using its service name and internal gRPC port.

Command:

    docker run --rm --network cw2-lab_peers --entrypoint /tmp/grpcprobe -v /tmp/grpcprobe:/tmp/grpcprobe:ro cw2-lab-node-a:latest node-b:9090

Result:

    RPC succeeded: node_id=node-b files=0

The HTTP remote-files endpoint on node-a was also tested:

    curl -i --max-time 10 'http://127.0.0.1:8081/v1/remote-files?peer_id=node-b'

Response:

    HTTP/1.1 200 OK
    {"node_id":"node-b","files":[]}

This confirms that node-a successfully requested file metadata from
node-b through the internal gRPC service.

## Storage isolation acceptance test

A synthetic 42-byte file named week08-isolation-test.txt was uploaded
to node-a.

Upload result:

    HTTP 201 Created

    {"event_delivery":{"delivered":3,"dropped":0,"duplicate":false},"file":{"name":"week08-isolation-test.txt","size":42,"sha256":"32a0098e42a1c52b9e29946ac95a6f939e1f734567b951c2b570b1099ba7c7cc","modified_unix":1795254317}}

The same filename was requested from node-b:

    curl -i --max-time 10 http://127.0.0.1:8082/v1/files/week08-isolation-test.txt

Response:

    HTTP/1.1 404 Not Found
    {"error":"file_not_found"}

This confirms that the file uploaded to node-a was not present in
node-b's separate storage.

## Peer stop and recovery acceptance test

Node-b was stopped:

    docker compose stop node-b

Result:

    Container cw2-lab-node-b-1 Stopped

The remote metadata request from node-a was then repeated:

    curl -i --max-time 10 'http://127.0.0.1:8081/v1/remote-files?peer_id=node-b'

Response while node-b was stopped:

    HTTP/1.1 503 Service Unavailable
    {"error":"peer_unavailable"}

Node-b was restarted:

    docker compose start node-b

Result:

    Container cw2-lab-node-b-1 Started

The Docker Compose status check showed all three containers running
and healthy, including node-b.

The remote metadata request was repeated:

    curl -i --max-time 10 'http://127.0.0.1:8081/v1/remote-files?peer_id=node-b'

Response after node-b restarted:

    HTTP/1.1 200 OK
    {"node_id":"node-b","files":[]}

This confirms that node-a could contact node-b again after its restart.

## Additional acceptance results

Remote gRPC metadata request: PASSED

Storage isolation between node-a and node-b: PASSED

Unavailable-peer response when node-b stopped: PASSED

Remote communication recovery after node-b restarted: PASSED

## Docker test and acceptance build targets

Both Docker test targets were built successfully from the submission root.

Standard test target:

    docker build --target test -f Dockerfile .

Result: BUILD FINISHED successfully.

The target ran:

    CGO_ENABLED=1 go test -race ./... -count=1 -timeout 60s

Acceptance test target:

    docker build --target acceptance -f Dockerfile .

Result: BUILD FINISHED successfully.

The target ran:

    CGO_ENABLED=1 go test -race -tags acceptance ./... -count=1 -timeout 120s

Both targets completed without reported test or build errors.

Go build image resolved during these builds:

    golang:1.27.3-bookworm

Resolved image digest:

    sha256:dc0963f32053e4bc451d24d87e3892d42d01021b61f093e98eb95c0352bb84b0

The digest records the image used for these local builds. Confirmation
that this image version is approved by the teaching team remains
outstanding.
