# Week 5 dependency and toolchain record

## Runtime dependencies

- `google.golang.org/grpc v1.75.1`
- `google.golang.org/protobuf v1.36.6`

These versions are pinned for coursework compatibility; this record does not claim they are the newest releases or constitute a production security approval. Indirect dependencies are recorded in `go.mod` and checksums in `go.sum`.

## Code-generation tools

- Go: `go1.27.1 darwin/amd64`
- Protocol Buffers compiler: `libprotoc 36.2`
- Go protobuf generator: `protoc-gen-go v1.36.6`
- Go gRPC generator: `protoc-gen-go-grpc 1.5.1`

The `.proto` source and generated Go bindings are included in the project. Run `scripts/generate.sh` on a machine with `protoc` installed to regenerate the bindings. The PowerShell equivalent is `scripts/generate.ps1`. Generated files and dependency checksums should be committed; local generator binaries and caches should not.

## Verification recorded

- `go build ./...` passed.
- `go test ./... -count=1 -timeout 20s` passed.
- `go test -race ./... -count=1 -timeout 30s` passed.
- `go vet ./...` completed without reported problems.
- `go mod verify` reported that all modules were verified.
