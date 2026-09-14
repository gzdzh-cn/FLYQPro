# FlyQPro 发布验证报告

版本：`desktop-v0.1.2-stable`  构建：`local-2026-09-12`  日期：`2026-09-12`  验证人：`Codex 本机自动化`

本轮交付定义：macOS arm64 本机可用、未签名的桌面稳定版。机器可读汇总见 [`desktop-stable-v0.1.2.json`](release-evidence/desktop-stable-v0.1.2.json)；Developer ID、公证、Windows、Android 真机仍按 `blocked` 记录。

## 当前自动化基线（2026-09-12）

- [x] `go test ./... -count=2 -shuffle=on -timeout=180s`
- [x] `go test -race ./internal/chat/... -timeout=180s`
- [x] `npm run build`
- [x] `./gradlew testDebugUnitTest assembleDebug`
- [x] Android 好友关系分发门禁单测（业务帧默认拒绝，未知帧 fail-closed）
- [x] `./gradlew -PvalidationBuild=true assembleDebugAndroidTest`（隔离 validation applicationId）
- [x] Desktop/Android `git diff --check`
- [x] 临时 macOS 验证二进制 `LC_BUILD_VERSION minos 11.0`，源 `Info.plist` 为 11.0.0
- [x] 本机 IPv4 loopback 100 MiB × 5 TLS/TCP-v3，最终 SHA-256 全部通过（中位 84.86 MiB/s；仅自动化证据）
- [ ] 真机、隔离网络、Windows、签名与公证验收（等待外部环境）

ADB 检查结果：当前无已连接设备，因此未执行安装和 instrumentation；既有 OPPO `Failure [-99]` 仍为外部阻断，不能以 APK 构建代替真机通过结论。

备注：macOS 链接阶段仍提示部分本机构建对象来自 macOS 26 SDK；临时验证二进制的 `minos` 已是 11.0，但仓库中既有 `.app` 仍是旧产物（Info.plist 为 10.13、adhoc 签名），必须重新打包，并以发布产物的 `otool -l`、签名和安装结果为准。

自动化原始证据：`docs/release-evidence/transfer-validation-100m-local.json`。该 loopback 结果不填写下方真实双机矩阵，也不替代 Android、隔离网络或安装签名证据。

## 环境

| 端 | 设备/系统 | 网络 | 安装包/签名 | 结果 |
| --- | --- | --- | --- | --- |
| macOS | 待填写 | 待填写 | Developer ID、公证、stapling | 等待外部验收 |
| Windows | 待填写 | 待填写 | NSIS、MSI、Authenticode | 等待外部验收 |
| Android | 待填写 | 待填写 | APK/AAB、SAF、通知权限 | 等待外部验收 |

## 传输与故障矩阵

每个场景至少记录 5 次；QUIC 与 TLS/TCP 必须使用相同硬件、文件和网络条件。

| 场景 | 方向/传输 | 大小 | 吞吐中位数 | 首包 p95 | ACK p95 | CPU | 峰值内存 | 磁盘写入 p95 | 恢复耗时 | 最终状态 | SHA-256 | 结果 |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- | --- |
| 基线单文件 | 双向 TLS/TCP | 100 MiB |  |  |  |  |  |  |  |  |  | 待验 |
| 基线单文件 | 双向 TLS/TCP | 1 GiB |  |  |  |  |  |  |  |  |  | 待验 |
| 基线单文件 | 双向 TLS/TCP | 10 GiB |  |  |  |  |  |  |  |  |  | 待验 |
| 进程重启 | TLS/TCP | 1 GiB |  |  |  |  |  |  |  | recovery-required/completed |  | 待验 |
| 网络切换 | TLS/TCP | 1 GiB |  |  |  |  |  |  |  | recovery-required/completed |  | 待验 |
| 锁屏/强杀 | Android ↔ 桌面 | 1 GiB |  |  |  |  |  |  |  | recovery-required/completed |  | 待验 |
| 磁盘不足 | 接收端 | 100 MiB |  |  |  |  |  |  |  | failed | 不适用 | 待验 |
| 源文件变化 | 发送端 | 1 GiB |  |  |  |  |  |  |  | failed | 不适用 | 待验 |
| 数据损坏 | 双向 | 100 MiB |  |  |  |  |  |  |  | failed | 必须拒绝 | 待验 |
| QUIC 实验 | 双向 QUIC | 1 GiB |  |  |  |  |  |  |  | completed | 必须一致 | 待验 |

## 安装、升级与数据

- [ ] macOS `Info.plist`、`MACOSX_DEPLOYMENT_TARGET` 与 `otool -l` 均为 11.0。
- [ ] macOS Developer ID 签名、公证与 stapling 通过。
- [ ] Windows NSIS/MSI 安装、覆盖升级、卸载保留数据、重装恢复通过。
- [ ] 发布签名任务在缺少凭据时 fail-closed；EXE、NSIS、MSI 和 `.app` 均完成签名验证。
- [x] macOS/Windows Taskfile 发布签名任务已配置为缺少凭据时 fail-closed（实际签名/公证仍待外部主机）。
- [ ] Android 前台服务、通知权限、锁屏、强杀、Wi-Fi 切换、SAF 持久授权通过。
- [ ] 旧数据库可读取；迁移中断可回滚；备份可恢复；消息与附件无丢失。

## 发布阻断审计

以下任一项为“是”时禁止发布：

| 阻断项 | 是/否 | 证据/日志 |
| --- | --- | --- |
| ACK 早于文件和 checkpoint 持久化 |  |  |
| 恢复范围超过 `.part` 实际长度 |  |  |
| 旧 session/generation ACK 被接受 |  |  |
| 重启后活动任务丢失 |  |  |
| 数据迁移、升级或签名失败 |  |  |
| QUIC 恢复/校验可靠性低于 TLS/TCP |  |  |

## QUIC 默认候选结论

- 恢复与校验成功率：`待填写`（要求 100%）
- 中位吞吐收益：`待填写`（要求至少 5%）
- 首包 p95 变化：`待填写`（退化不得超过 5%）
- 结论：`继续实验 / 可进入默认候选`
