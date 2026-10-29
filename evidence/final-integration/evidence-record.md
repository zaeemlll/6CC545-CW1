# Final Integration Evidence Record

**Student number:** 100874830
**Repository:** https://github.com/zaeemlll/6CC545-CW1
**Current integration commit recorded:** `9815ec6c5132d0f08468df0366ec494ced508489`
**Final tag:** Not yet created
**Environment recorded:** macOS 14.5, Go 1.27.1, darwin/amd64

## Integrated scope
The cumulative project includes:
- Node configuration and `/healthz`.
- JSON REST status operations at `/v1/status`.
- Peer registration and local file metadata.
- TCP and UDP messaging and the `/v1/messages` endpoint.
- gRPC remote file metadata and `/v1/remote-files`.

## Recorded verification
The existing `test-results.txt` records successful project checks, build, full Go tests, race tests, `go vet`, module verification and `git diff --check` on 21 October 2026. It reports that all modules were verified.

## Failure cases previously recorded
- Invalid JSON: HTTP 400.
- Duplicate peer registration: HTTP 409.
- Unavailable remote gRPC peer: HTTP 503 with `peer_unavailable`.
- Stopped messaging peer: connection or response failure.

## Remaining submission work
The root evidence records have now been drafted, but they still need to be checked against the assessment brief and the original checkpoint evidence. The commentary PDF, root implementation log, required architecture artefact and final submission archive also need to be checked or completed. Do not create the `final-cw1` tag until the final snapshot is ready and verified.

## Evidence integrity note
This record summarises previously supplied outputs and repository information. It does not claim that all checks were rerun at the current integration commit.
