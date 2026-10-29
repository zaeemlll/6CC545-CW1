# Week 2 Evidence Record

**Student number:** 100874830
**Repository:** https://github.com/zaeemlll/6CC545-CW1
**Checkpoint tag:** `week02`
**Checkpoint commit:** `01f69e614da67e4ecd50e9b6950154c608902f80`
**Implementation commit:** `d4d10e29e5c65af1fa8bfeb08655b5edea7e68cf`

## Aim
Implement a JSON REST status resource using HTTP GET and POST.

## Observed behaviour previously recorded
- GET `/v1/status` returned HTTP 200 with the current status.
- POST could update the status to `degraded`, and a subsequent GET showed the updated value.
- Malformed JSON returned HTTP 400 with `invalid_json`.
- The request body has a 4096-byte limit.

## Design decision
The endpoint uses JSON request and response bodies and validates incoming data.

## Verification
The repository's test-results record includes a later successful full test run. Preserve any original Week 2 terminal output as additional evidence if available.

## Limitations
The status is simulated application state, not a measurement of the physical machine's health.
