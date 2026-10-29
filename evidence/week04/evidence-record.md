# Week 4 evidence record

**Student number:** 100874830

**Week:** Week 4

**Repository URL:** https://github.com/zaeemlll/6CC545-CW1.git

**Immutable tag:** `week04`

**Commit hash:** `5f83cac10700f86db11963cb675fa73d5f930f3d` (checkpoint tag target; implementation commit: `71499ae4952ae27b4d76c571961e8132c161ef58`)

**Go version / OS:** `go1.27.1 darwin/amd64`

## Commands actually executed

- `go test ./internal/ipc -run "^TestWeek04_" -v -count=1`
- `go test ./... -count=1 -timeout 20s`
- `go test -race ./... -count=1 -timeout 30s`
- `go vet ./...`
- Live TCP and UDP client requests between Node A and Node B.
- Live GET requests to `/v1/messages` on both nodes.

## Observed results (do not paste expected examples as results)

- Targeted Week 4 IPC tests passed, covering TCP and UDP round trips, oversized TCP messages, UDP timeout/drop behaviour, cancelled clients, fragmented TCP frames, frame-size limits and invalid messages.
- The full Go test suite passed.
- The race detector passed without reported race warnings.
- `go vet ./...` completed without reported problems.
- TCP and UDP messages were accepted in both directions between Node A and Node B.
- Both `/v1/messages` endpoints returned HTTP 200 and showed received messages.
- With Node B stopped, the TCP client reported connection refused. The UDP client also reported connection refused while waiting for a response in this local test.

## Failure case and diagnosis

Stopping Node B prevented successful messaging. TCP reported connection refused when the client attempted to connect. The UDP client reported connection refused while waiting for a reply in this local environment. UDP does not guarantee delivery, and behaviour when a destination is unavailable can depend on the operating system.

## Design decision and alternative

Messages use validated JSON. TCP uses a four-byte length prefix to identify message boundaries because TCP provides a byte stream rather than separate application messages. UDP sends each message as a datagram and preserves datagram boundaries. Both transports return acknowledgements when the receiver accepts a message. Accepted messages are stored in a bounded in-memory inbox exposed through `GET /v1/messages`.

TCP provides reliable, ordered byte-stream delivery, but it needs framing to identify complete messages. UDP has less framing overhead and sends individual datagrams, but it does not guarantee delivery or ordering. UDP datagrams may also be duplicated. TCP is suitable when reliable, ordered delivery is important, while UDP demonstrates lightweight message exchange when the application can handle possible message loss. The tests demonstrate the basic behaviour of both transports but do not provide a formal performance benchmark.

A database or durable message queue could retain messages across restarts, but an in-memory inbox is sufficient for this checkpoint.

## Remaining limitations

The inbox is stored in memory and is not preserved after a node restarts. UDP can lose, duplicate or reorder messages. Acknowledgements can help the sender identify successful receipt, but they do not automatically guarantee delivery in every failure scenario. The local teaching service does not include authentication or encryption and should not be exposed to an untrusted network.

## Sources and AI-use disclosure

I consulted the Week 4 coursework guidance, the supplied project scaffold, the Go documentation, RFC 9293 for TCP and RFC 768 for UDP. ChatGPT assisted with explanations and troubleshooting. I ran the tests and live requests and checked the observed results.
