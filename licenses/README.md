# Third-party licenses

CubeDock itself is MIT licensed; see the repository `LICENSE`. This directory
contains the license texts and attribution for third-party components embedded
in CubeDock binaries and the embedded web console.

## License texts

- `MIT.txt` — MIT License (CubeDock and MIT components)
- `BSD-3-Clause.txt` — BSD 3-Clause License
- `Apache-2.0.txt` — Apache License 2.0

Vendored license files are copied next to this README under `vendored/` when a
release package is built.

## Components

### Authentication

CubeDock's NetEase authentication chain (`internal/netease`, `internal/auth`,
`internal/interoplogin`) is a first-party implementation. The historical
`vendor-src/g79client` and `internal/funauth` AGPL components were removed on
2026-11-11.

### Networking and protocol

| Component | Location | License / copyright |
| --- | --- | --- |
| gophertunnel (patched local copy) | `limbo/third_party/gophertunnel` | MIT, Copyright (c) 2019 Sandertv |
| go-raknet fork | `vendor-src/nemc-tan-lobby-solver/core/raknet` | MIT, Copyright (c) 2019 Sander ten Veldhuis |
| Dragonfly NetherNet | `vendor-src/nemc-tan-lobby-solver/core/nethernet` | MIT, Dragonfly Tech |
| Pion WebRTC/SCTP/datachannel | `vendor-src/nemc-tan-lobby-solver/core/{webrtc,sctp,datachannel}` | MIT, Pion contributors |
| gophertunnel protocol/packet | `vendor-src/nemc-tan-lobby-solver/{minecraft,protocol}` | MIT, Copyright (c) 2019 Sandertv |

The `nemc-tan-lobby-solver` root and `bunker/` were reworked/removed by
CubeDock; only subdirectories carrying their own MIT LICENSE are used.

### Go modules

| Module | License |
| --- | --- |
| `github.com/sandertv/go-raknet` | MIT |
| `github.com/coder/websocket` | MIT |
| `github.com/df-mc/jsonc` | MIT |
| `github.com/go-gl/mathgl` | MIT |
| `github.com/ugorji/go/codec` | MIT |
| `github.com/andreburgaud/crypt2go` | MIT |
| `github.com/database64128/chacha8-go` | MIT |
| `github.com/klauspost/compress` | BSD-3-Clause |
| `github.com/google/uuid` | BSD-3-Clause |
| `golang.org/x/crypto`, `golang.org/x/net`, `golang.org/x/sys`, `golang.org/x/term` | BSD-3-Clause |
| `github.com/go-jose/go-jose/v4` | Apache-2.0 |
| Pion modules (`dtls`, `stun`, `ice`, `srtp`, `rtcp`, `rtp`, `sdp`, `interceptor`, `mdns`, `transport`, `turn`, `logging`, `randutil`) | MIT |

Copyright notices are retained in each module's LICENSE file. Use
`go list -m all` for the exact version graph.

### Frontend

React, React DOM, Material UI, MUI Icons, Emotion and Vite are MIT licensed.
Their production bundle is embedded into the Go binary; no external CDN is
used at runtime.

### Game names and trademarks

Minecraft, Mojang, Microsoft, NetEase and related names/trademarks belong to
their respective owners. CubeDock does not ship a game client, server software
or official game assets.

## Distribution

When distributing a CubeDock binary, include this directory (and the vendored
license files) together with `LICENSE`, `NOTICE` and
`THIRD_PARTY_NOTICES.md`. No AGPL source offer is required for the current
authentication implementation.

This is an engineering record, not legal advice; have counsel review before a
public distribution.
