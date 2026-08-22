# BitTorrent v1 Client

A from-scratch Go implementation of the core BitTorrent v1 download path. It
is intended as a protocol-learning and systems project: metainfo parsing,
tracker discovery, peer-wire communication, pipelined block requests, and
piece-integrity verification are explicit rather than hidden behind a library.

## Run

```bash
go run . <file.torrent> <output-path>
```

Example:

```bash
go run . ubuntu.iso.torrent ./ubuntu.iso
```

The program prints the metainfo summary, announces to the configured tracker,
connects to available peers, then reports completed pieces and download rate.

## Download path

```text
.torrent bytes
    │
    ▼
bencode decoder ──► raw `info` bytes ──► SHA-1 info hash
    │
    ▼
HTTP tracker announce ──► compact IPv4 peer addresses
    │
    ▼
TCP peer handshake ──► bitfield / interested / unchoke
    │
    ▼
Pipelined 16 KiB requests ──► piece blocks
    │
    ▼
Piece SHA-1 verification ──► WriteAt(output, piece offset)
```

## Package map

| Package | Responsibility |
| --- | --- |
| `bencode` | Decodes and encodes integers, strings, lists, and dictionaries. `ValueRange` preserves the original raw `info` dictionary bytes for correct info-hash calculation. |
| `torrent` | Parses a single-file v1 metainfo file into name, length, piece length, hashes, announce URL, and info hash. |
| `tracker` | Sends an HTTP compact announce and decodes IPv4 peer endpoints. |
| `peer` | Implements the TCP handshake, length-prefixed peer-wire messages, bitfields, interested/unchoke flow, block requests, and piece SHA-1 checks. |
| `download` | Verifies an existing output file for resume, owns `pending → in-flight → verified` piece state, assigns a piece to only one eligible peer at a time, retries failed leases, and writes verified pieces at their correct offset. |

## Current implementation details

- BitTorrent v1 `info_hash`: `SHA1(raw bencoded info dictionary)`
- Tracker mode: HTTP, compact IPv4 peers
- Download mode: single-file torrents
- Maximum peer workers: `50`
- Block size: `16 KiB`
- Per-peer request window: `15` outstanding blocks
- Piece integrity: SHA-1 verified before writing
- Output writes: random access via `WriteAt`
- Resume: existing output pieces are SHA-1 checked; only verified pieces are skipped
- Scheduler: a piece remains pending until one eligible peer leases it, then becomes verified only after the writer commits it
- Peer-wire safety: a `2 MiB` inbound message limit and exact requested-block validation

## Current scope and limitations

This is not yet a complete production torrent client. It does not currently
support multi-file torrents, tracker tiers, UDP trackers, DHT, magnets, IPv6
peers, uploading/seeding, or extension protocols. A partially written
single-file output can now be resumed after piece verification, but there is
no separate persistent resume metadata yet.

Next priorities are tracker tiers and periodic reannounce, dynamic peer
availability updates, safe endgame cancellation, and multi-file metainfo.
The automated suite currently covers piece-scheduler lease/retry behavior,
resume verification, a local HTTP tracker response, compact-peer parsing,
handshake exchange, bitfield plus unchoke processing, block requests, SHA-1
checks, and oversized-message rejection.

## Verification

```bash
go test ./...
go vet ./...
go build ./...
```
