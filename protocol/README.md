# dzhgo protocol v2

FlyQPro uses a language-neutral JSON protocol. The application brand remains
FlyQPro; `dzhgo` is the canonical communication protocol name.

## Protocol identity

`dzhgo` is the only supported protocol dialect. The protocol major version is `2`, and discovery uses `DZHGO_DISCOVERY_V1`.

Every discovery request, announcement, TLS hello, and response must use the exact tuple `dzhgo` / `DZHGO_DISCOVERY_V1` / `2`. An unknown protocol name, magic value, major version, or unmet `minMajor` is rejected. There is no protocol-name fallback.

Peers persist the negotiated `protocolName`, `protocolMajor`, `discoveryMagic`, and `capabilities`; valid peers are always recorded as the canonical dzhgo/v2 dialect.

## Discovery

- Transport: IPv4 UDP broadcast.
- Port: `39190`.
- Fallback: TCP `39190` probes local IPv4 subnets when UDP broadcast is unavailable.
- Encoding: UTF-8 JSON.
- Discovery messages never carry chat or file content.

Example announcement:

```json
{
  "magic": "DZHGO_DISCOVERY_V1",
  "type": "announce",
  "protocol": "dzhgo",
  "major": 2,
  "minor": 0,
  "minMajor": 2,
  "deviceId": "sha256-of-public-key",
  "nickname": "Alice",
  "avatarHash": "sha256-of-avatar",
  "avatarVersion": 1,
  "platform": "macOS",
  "osVersion": "darwin arm64",
  "port": 42000,
  "publicKey": "base64-or-pem-public-key",
  "certificateFingerprint": "sha256-of-x509-der",
  "capabilities": ["text", "image", "file", "file-progress-v1", "attachment-demand-v1"]
}
```

## Chat connection

- Transport: TCP with TLS 1.2 or newer.
- Control frames: newline-delimited UTF-8 JSON.
- Files: base64 encoded 32 KiB JSON chunks.
- Device identity: ECDSA P-256 public key SHA-256.
- Certificate fingerprint: SHA-256 of the DER encoded X.509 certificate.

The first frame is `hello`; the peer answers with `hello_ack` using the same
negotiated dialect. Friend requests and messages are rejected until the local
database marks the device as a friend. Unknown optional frames and fields may be ignored by other dzhgo/v2 clients, while text, image, and ordinary file behavior remains stable.

The capabilities `avatar-sync-v1`, `file-progress-v1`, `attachment-demand-v1`, `offline-v1`, and `friend-restore-v2` are optional dzhgo/v2 features and are used when advertised by both sides. With `attachment-demand-v1`, a `file_offer` carries metadata and an optional sender-generated thumbnail first; the original file is sent only after `file_accept`. Older peers continue using the direct-transfer behavior.

When a FlyQPro service stops normally, it may broadcast an optional `offline`
discovery frame. Receivers keep friends in the list and mark them offline;
non-friends may be removed from the discovery list. Unexpected exits and
network failures are detected by authenticated TLS health probes.

## Friend restore signatures

A friend that still has the original authenticated device key may send an optional `friend_restore` frame after TLS authentication. The signature domain is fixed to:

```text
dzhgo/friend-restore/v1
sourceDeviceId
targetDeviceId
sourcePublicKey
```

The receiver validates the TLS peer, target ID, public-key-derived source ID, and the dzhgo signature. No friend request, notification, message, or unread count is created by restoration.

## Optional fields and message behavior

The following fields are optional and may be ignored by other dzhgo/v2 clients:

- `avatarHash`, `avatarVersion`, `avatarMime`, `avatarData`.
- `capabilities`, `messageIds`, `syncSince`, `syncUntil`, `syncToken`, `readAt`.
- `acceptedAt` on `friend_request_response`.
- `targetDeviceId`, `sourceDeviceId`, `sourcePublicKey`, `restoreVersion`,
  `restoreSignature`.
- `offline` discovery presence events and `probe` hello flags.
- `attachmentId`, `fileName`, `mimeType`, `fileSize`, `sha256`, `chunkIndex`,
  `thumbnailData`, `thumbnailMime`, `chunkIndex`, and `payload` for encrypted
  attachment transfer.
- `file_progress` frames and the `file-progress-v1` capability.

Demand-based attachment control frames are `file_accept`, `file_reject`, and
`file_cancel`. A pending offer must not create a receiver `.part` file or emit
progress. The receiver may accept to the default attachment directory, choose
another path, or reject the offer. Either side may cancel while waiting or
transferring; the unfinished temporary file is then removed. A thumbnail is
base64 encoded, limited to about 128 KiB, and is intended for image previews
only; ordinary files use their name, size, and file icon.

Message status values are `sending`, `sent`, `read`, and `failed`. A receiver
must acknowledge a duplicate `messageId`, but must not persist or display it
again.

## Versioning and errors

The dzhgo protocol name and discovery magic are fixed. Breaking wire changes increment `major` and return `VERSION_TOO_OLD` or `PROTOCOL_UNSUPPORTED`.

Error codes include `PROTOCOL_UNSUPPORTED`, `VERSION_TOO_OLD`,
`CERTIFICATE_CHANGED`, `DEVICE_KEY_CHANGED`, `DEVICE_NOT_TRUSTED`, and
`FRIENDSHIP_REQUIRED`.
