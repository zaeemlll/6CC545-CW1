# Week 5 Dependency and Toolchain Record

## Runtime dependencies

- `google.golang.org/grpc v1.75.1`
- `google.golang.org/protobuf v1.36.6`

These versions are pinned for coursework compatibility. This record does not claim that they are the newest releases or that they have received a production security assessment. Indirect dependencies are recorded in `go.mod`, with checksums stored in `go.sum`.

## Code-generation tools

- Go: `go1.27.1 darwin/amd64`
- Protocol Buffers compiler: `libprotoc 36.2`
- Go protobuf generator: `protoc-gen-go v1.36.12`
- Go gRPC generator: `protoc-gen-go-grpc 1.6.2`

The `.proto` source and generated Go bindings are included in the project. The generation script is `scripts/generate.sh`; it requires the relevant Protocol Buffers compiler and Go generators to be installed and available. Generated files and dependency checksums should be committed, while local generator binaries and caches should not.

## Verification recorded

The following commands were run from the `source` directory:

- `go build ./...` — completed without build errors.
- `go test ./... -count=1 -timeout 20s` — all packages with tests passed.
- `go test -race ./... -count=1 -timeout 30s` — all packages with tests passed.
- `go vet ./...` — completed without reported problems.
- `go mod verify` — reported `all modules verified`.
