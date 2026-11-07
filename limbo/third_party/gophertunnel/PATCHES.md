# Local gophertunnel patches

This is a copy of `github.com/sandertv/gophertunnel` v1.53.0 (MIT, see
`LICENSE`). It is used only by the `limbo` module via a `replace` directive.

## Client resource-pack completion hook (`minecraft/dial.go`, `minecraft/conn.go`)

`Dialer.OnResourcePacksComplete` is called after the client downloads all packs
and receives `ResourcePackStack`, immediately before the `Completed` response.
The pack puller uses it to save the packs, send a RakNet `DisconnectNotification`
(0x15), and return an error that aborts login. This leaves the backend before
the Bedrock spawn sequence, so Geyser/Java login plugins see the bot disconnect
before it enters the world. `ResourcePacks()` now copies under `packMu`.

## Lenient offline login (`minecraft/protocol/login/request.go`, `minecraft/conn.go`)

Offline limbo does not use the client's skin/device data. Some NetEase clients
send custom skins that fail `ClientData.Validate`, or use NetEase display names
longer than Mojang's 16-byte offline limit (Chinese names are 3 bytes per
character), which previously closed the login without a response.
`login.ParseLenient` verifies the single-segment certificate signature but
skips client-data and display-name/expiry validation. `handleLogin` uses it when
authentication is disabled, keeping the identity and public key needed for the
encryption handshake.

## NetEase identity claims (`minecraft/protocol/login/data.go`)

`login.IdentityData` has two extra optional JSON fields, `netease_uid` and
`netease_uuid`, so the offline login used by the pack puller carries the
NetEase identity claims expected by NetEase-compatible servers.

## Tolerate trailing bytes (`minecraft/packet.go`)

When `disconnectOnInvalidPacket` is disabled, trailing bytes after a known
packet are ignored instead of failing the whole connection. NetEase servers add
extra bytes to some packets (observed on `ResourcePackStack`), and the pack
puller enables this non-strict mode.

## NetEase resource pack workaround (`minecraft/conn.go`)

NetEase clients acknowledge `ResourcePacksInfo` with `Completed` or `Refused`
without requesting the advertised packs. To make the client download and apply
the packs before `StartGame`, the server-side login flow is patched to:

1. Send an empty `ResourcePacksInfo` (no texture pack list);
2. Rewrite the client's first `Completed`/`Refused` response into
   `PacketResponseSendPacks` containing every configured pack;
3. Guard `ResourcePackStack` with `packStackSent` so the normal
   `AllPacksDownloaded` path and the fallback timer cannot send it twice;
4. After the last pack chunk, wait 3 seconds and send `ResourcePackStack`
   ourselves if the client did not send `AllPacksDownloaded`.

The workaround is controlled by `ListenConfig.ResourcePackForceDownload`. limbo
enables it by default (`-packs-mode netease`) and disables it for local
standard-client testing (`-packs-mode standard`).

The approach follows Happy2018new's `pre-transfer-server`; no code was copied
from that project. Keep this patch small and rebase it when gophertunnel is
upgraded.
