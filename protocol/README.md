# POPChat protocol v1

POPChat uses a language-neutral protocol so another desktop or mobile client can interoperate without depending on Wails bindings.

## Discovery

- Transport: IPv4 UDP broadcast.
- Port: `39190`.
- Fallback: TCP `39190` probes the local IPv4 subnets when UDP broadcast is unavailable.
- Encoding: UTF-8 JSON.
- Magic: `POPCHAT_DISCOVERY_V1`.
- Discovery messages never carry chat or file content.

Example announcement:

```json
{
  "magic": "POPCHAT_DISCOVERY_V1",
  "type": "announce",
  "protocol": "POPChat",
  "major": 1,
  "minor": 0,
  "deviceId": "sha256-of-public-key",
  "nickname": "Alice",
  "avatarHash": "sha256-of-avatar",
  "avatarVersion": 1,
  "platform": "macOS",
  "osVersion": "darwin arm64",
  "port": 42000,
  "publicKey": "base64-or-pem-public-key",
  "certificateFingerprint": "sha256-of-x509-der",
  "capabilities": ["text", "image", "file"]
}
```

## Chat connection

- Transport: TCP with TLS 1.2 or newer.
- Control frames: newline-delimited UTF-8 JSON.
- Files: base64 encoded 32 KiB JSON chunks in v1; a later major version may add a binary frame while retaining the control schema.
- Device identity: ECDSA P-256 public key SHA-256.
- Certificate fingerprint: SHA-256 of the DER encoded X.509 certificate.

The first frame is `hello`; the peer answers with `hello_ack`. Friend requests and messages are rejected until the local database marks the device as a friend.

After a friend handshake, a client may send `avatar_request`. The friend may answer with an optional `avatar_response` containing `avatarHash`, `avatarVersion`, `avatarMime`, and base64 `avatarData`. Avatar bytes are never sent through discovery announcements and are accepted only after the certificate-verified TLS handshake and friendship check.

The following fields are optional and may be ignored by older clients:

- `avatarHash`, `avatarVersion`, `avatarMime`, `avatarData`: profile avatar cache negotiation and transfer.
- `capabilities`: supported message kinds such as `text`, `image`, and `file`.
- `messageIds`: message IDs carried by `read_receipt`.
- `syncSince`, `syncUntil`, `syncToken`, `readAt`: optional message synchronization and read-state hints.
- `attachmentId`, `fileName`, `mimeType`, `fileSize`, `sha256`, `chunkIndex`, and `payload`: encrypted attachment transfer metadata and chunks.

Message status values exposed by the application are `sending`, `sent`, `read`, and `failed`. A receiver must acknowledge a duplicate `messageId`, but must not persist or display it again.

## Versioning

Unknown fields must be ignored. New optional fields are backward compatible. A breaking change increments `major` and returns `VERSION_TOO_OLD` or `PROTOCOL_UNSUPPORTED`.

## Error codes

`PROTOCOL_UNSUPPORTED`, `VERSION_TOO_OLD`, `CERTIFICATE_CHANGED`, `DEVICE_NOT_TRUSTED`, `FRIENDSHIP_REQUIRED`.
