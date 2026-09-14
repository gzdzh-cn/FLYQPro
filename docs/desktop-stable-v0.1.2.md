# FlyQPro Desktop Stable v0.1.2

状态：本机可用稳定版（macOS arm64，未签名）。

## 本版本范围

- macOS 11.0 及以上。
- TLS/TCP v3 为唯一默认文件传输协议。
- SQLite/sidecar 恢复、ACK 持久化顺序、重复 EndFile 幂等和桌面长文本折叠已纳入验收。
- 自动化测试、100 MiB TLS/TCP loopback 传输和报告格式校验必须通过。

## 不属于本版本阻断项

Android 真机/SAF、Windows 安装器、Developer ID/公证、QUIC、数据库敏感字段加密列入后续版本；本文件不将它们标记为已完成。

## 构建状态

- 版本源：`internal/version/version.go`、`build/config.yml`、`build/darwin/Info.plist`。
- 当前交付包：本机 macOS arm64 `.app`/DMG。
- 签名状态：未签名；公开发布仍需 Developer ID、公证和 stapling。
