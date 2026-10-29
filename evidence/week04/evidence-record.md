# Week 4 Evidence Record

**Student number:** 100874830
**Repository:** https://github.com/zaeemlll/6CC545-CW1
**Checkpoint tag:** `week04`
**Checkpoint commit:** `5f83cac10700f86db11963cb675fa73d5f930f3d`
**Implementation commit:** `71499ae4952ae27b4d76c571961e8132c161ef58`

## Aim
Implement inter-node messaging using TCP and UDP.

## Observed behaviour previously recorded
- TCP and UDP messages were exchanged between the two local nodes.
- The `/v1/messages` endpoint returned HTTP 200 when queried.
- When the receiving node was stopped, connection or response failures were observed, depending on the transport and operating-system behaviour.

## Design decision
The project demonstrates both connection-oriented TCP and datagram-based UDP messaging. The protocol documentation describes message framing and acknowledgements.

## Documentation
See `source/docs/api-and-protocols.md`, section "Week 4 — TCP and UDP Inter-Node Messaging".

## Limitations
UDP does not guarantee delivery. A failed connection or timeout should be recorded as a failure case, not as a successful message exchange.
