# Week 8 – Storage Recovery Evidence

Student number: 100874830
Task: Local storage recovery and Docker volume persistence

## Test 1 – Local storage recovery

Node ID: cw2-recovery
HTTP address: 127.0.0.1:18081
Storage directory: /tmp/cw2-week08-recovery.DIaGpw

### Test results

- Liveness endpoint: HTTP 200 OK.
- Readiness endpoint: HTTP 200 OK.
- File upload: HTTP 201 Created.
- File metadata before restart: HTTP 200 OK.
- Node stopped gracefully using Ctrl+C.
- Node restarted using the same storage directory.
- Recovered file metadata: HTTP 200 OK.
- Recovered file download: HTTP 200 OK.
- Recovered content: Week 8 storage recovery test
- File size: 29 bytes.
- Original and recovered SHA-256:

    8a05085c7f65d8f6c7909be17112ed3cb65c12743baeddacc880319028c37f01

### Interpretation

The node recovered the stored file metadata from the same local
storage directory after a graceful shutdown and restart. The
original file content, size and SHA-256 hash were preserved.

## Test 2 – Docker named-volume persistence

Node ID: node-a
Container: cw2-lab-node-a-1
Storage volume: cw2-lab_data-a
Container storage path: /data
File: persistence-test.txt

### Test results

- File created with a size of 35 bytes.
- File uploaded to node-a: HTTP 201 Created.
- File downloaded before restart and matched the original.
- Node-a restarted using:

    docker compose restart node-a

- Node-a returned to healthy status.
- Node-b and node-c remained healthy.
- File downloaded again after the restart: HTTP 200 OK.
- Byte comparison between the original and recovered file succeeded.
- Recovered file size: 35 bytes.
- Original and recovered SHA-256:

    8246babb2456aa456df9ec9f5024cc7ca3562c86b1f9c94fabced72671f0f653

### Interpretation

The file remained available after node-a's container restarted.
The recovered content and SHA-256 matched the original, confirming
persistence through the node's named Docker volume.

## Storage volume isolation

Docker inspection confirmed separate volumes for the three nodes:

- node-a: cw2-lab_data-a mounted at /data.
- node-b: cw2-lab_data-b mounted at /data.
- node-c: cw2-lab_data-c mounted at /data.

## Limitations

The Docker test verifies persistence after a container restart.
It does not test persistence after deleting Docker volumes.

The tests do not establish file replication between the three nodes
or recovery from physical disk corruption.

## AI acknowledgement

ChatGPT was used for explanations and troubleshooting guidance.
The student ran the commands and checked the observed results.

## Storage isolation acceptance test

A synthetic file was uploaded to node-a:

    File: week08-isolation-test.txt
    Size: 42 bytes
    Upload result: HTTP 201 Created
    SHA-256: 32a0098e42a1c52b9e29946ac95a6f939e1f734567b951c2b570b1099ba7c7cc

The same file was requested from node-b:

    curl -i --max-time 10 http://127.0.0.1:8082/v1/files/week08-isolation-test.txt

Response:

    HTTP/1.1 404 Not Found
    {"error":"file_not_found"}

### Interpretation

The file was successfully stored on node-a but was not found on
node-b. This confirms that the nodes use independent storage and
that uploading a file to one node does not automatically copy it
to another node.
