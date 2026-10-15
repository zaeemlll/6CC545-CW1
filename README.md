# 6CC545 CW1 — Distributed Applications and Web Services

## Project structure

This is my cumulative coursework project for the 6CC545 Distributed Applications and Web Services module.

- `source/` — Go application source code, tests, scripts and build instructions.
- `evidence/` — Weekly evidence and test results.
- `commentary/` — Coursework commentary and design explanations.
- `implementation-log/` — Record of development decisions, tests and progress.
- `docs/` — Project-level documentation.
- `repository.txt` — Repository URL, to be completed when the remote repository is created.

## Current progress

Week 1 focuses on validating node configuration and providing an HTTP health endpoint. Two local nodes can run on separate loopback ports.

The Week 1 Go tests passed, and `go vet ./...` completed without reporting issues. The detailed evidence record is in `evidence/week01.md`.

## Running the project

Follow the build, test and run instructions in [`source/README.md`](source/README.md).

## Safety and limitations

The services currently have no TLS or authentication. They must remain bound to loopback and must not be exposed to other networks. The project will be extended in later weekly stages.
