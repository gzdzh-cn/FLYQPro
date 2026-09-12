# Link2Send 对标改造实施记录

## 已落地

- `dzhgo` v3 文档已统一为 v3 discovery/hello；文件数据明确使用 TLS/TCP 二进制帧，QUIC 仅作显式实验传输，不再静默回退 JSON/base64。
- Go hello 能力声明新增 `binary-transfer-v3`、`range-resume-v3`、`folder-manifest-v3`，旧版本不会被当作 v3 文件通道处理。
- Go/TypeScript/Android 传输进度增加 canonical `state`，并预留 session、generation、slot、重试、错误码、durable bytes 字段；保留旧 `phase` 以兼容现有 UI。
- 桌面服务新增 `ListTransferSessions` 和 `GetActiveTransferCount`，用于诊断和传输监控；Wails bindings 由生成任务自动更新。
- `transfer_resumes` 已增量扩充方向、session/generation、checkpoint、源文件时间、重试、错误码、可重试标记和 canonical state。接收端按“文件 fsync → SQLite/sidecar checkpoint → ACK”执行；两份记录只选择 checkpoint 最大且不越过 `.part` 长度的一份，不跨 checkpoint 合并范围。
- 新增 `transfer_resume_migrations` 迁移游标，记录 `pending`、`migrating`、`verified`、`rollback`；sidecar 原子替换后同步父目录。发送端每 4 MiB 及最终 ACK 持久化远端确认范围，暂停保留范围，完成/失败保留终态。
- 完成、失败和取消保留 SQLite 终态；最终提交增加目标文件和父目录同步。桌面新增活动传输、恢复任务、诊断、暂停、恢复、重试和取消接口。
- TLS 外连在 `VerifyConnection` 同时校验设备 ID、公钥和证书指纹；入站 v3 数据连接在读取 BeginFile 前以三项完整身份映射好友，不依赖证书 CN。身份错误使用结构化 `TransferError`。
- `clientTLSConfig` 现在强制要求目标 Peer 的三项完整 pin，不再允许空字段作为通配符。Android 同样拒绝身份材料缺失，并禁止发现消息静默覆盖已保存证书指纹。
- Android 每个 v3 chunk 在 `FileDescriptor.sync()` 和恢复记录写入后 ACK；失败、暂停、取消、完成保留恢复终态，启动时重建可恢复发送任务。
- Android 恢复任务列表包含 queued 和可重试 failed 任务，新增显式重试入口；`TransferProgress` 补齐 ISO-8601 `updatedAt` 并通过桌面 golden JSON 解码测试。
- 桌面前端已切换为消费 `transfer-progress` 等版本化事件；旧 `chat:transfer-progress` 仅由后端保留兼容，不再造成前端双重更新。
- 增加确定性故障连接夹具，可精确注入断连、读写延迟、丢弃、重复和单次写入损坏；旧 session/generation ACK 拒绝测试继续作为发布阻断回归项。
- Android 与桌面消息气泡支持最多 8 行折叠、真实布局溢出判断和展开/收起；完整内容仍用于复制、搜索、引用和持久化。
- 传输调参增加 EWMA、连续 3 个坏样本降速、连续 5 个好样本升速和调整后 2 个窗口冷却；QUIC 明确关闭 0-RTT，并要求 TCP/QUIC 各至少 5 次且全部成功后才允许候选切换，首包延迟允许的退化不超过 5%。
- 发送端远端确认范围现在与接收端一样执行 SQLite + sidecar 双写；进度事件继承恢复记录的 retryable 标记，避免错误分类与诊断状态不一致。
- Android QUIC 实验选择器与 Go 对齐：TCP/QUIC 均需至少 5 次且全部成功，首包 p95 退化超过 5% 或吞吐收益不足 5% 时继续使用 TLS/TCP。
- `RangeWriterV3` 原子提交后同步目标文件和父目录，避免 rename 目录项在崩溃后不可见。
- 接收持久化 I/O 已通过包内接口封装，测试可确定性注入 chunk fsync、SQLite、sidecar、rename、目标文件和目录同步失败；SQLite/sidecar 全失败时不会产生成功 ACK。
- 成功完成才清理 sidecar；失败终态继续写入 SQLite/sidecar，校验失败保留 `.part` 并清空可恢复范围。
- Android DocumentTree 恢复记录新增 tree URI、document ID、相对路径、持久授权和源文件元数据；恢复时重新解析 SAF 源并重新协商 generation。
- Android 启动恢复现在只自动加载 queued/暂停态及可重试 failed 的发送任务；接收方向任务显式保留为 `recovery-required`，不可重试终态、过期任务和 SAF 授权丢失任务不会被静默重传。
- 桌面发送恢复在加载 sidecar/SQLite 前强制校验源文件大小与 mtime；源文件变化会拒绝旧 checkpoint，并保留可诊断的失败结果。
- Android 诊断接口补齐活动任务、恢复任务及 TLS pool 的 session、活动/已连接 slot 和重连次数。
- Android 控制连接在业务帧分发前统一重新检查当前好友关系；好友申请/响应、删除同步和 ping 之外的消息、回执、文件恢复/取消、头像及共享盘操作全部默认拒绝，未知帧同样按需好友处理，避免仅凭 hello 自证明进入业务路径。
- 新增分层传输工作流：100 MiB PR、1 GiB 夜间、10 GiB 手动发布任务，默认各执行 5 次并生成 SHA-256 验证后的 JSON 报告。
- 大文件报告现在带有 `schemaVersion/status/buildId/commit/worktree` 及每条样本的 `result`；`hack/validate-release-evidence.py` 已接入三层 CI，缺少状态、构建信息或成功样本完整性时直接失败。
- macOS 最低版本已统一为 11.0；已形成存储与身份威胁模型，字段加密仍按稳定版之后的阶段实施。
- macOS `package:signed` 和 Windows `package:signed` 发布任务已加入 fail-closed 凭据检查；macOS 任务执行 Developer ID/公证/stapling，Windows 任务对 EXE、NSIS 和 MSI 执行 Authenticode 验证，实际签名仍需对应主机与凭据。
- 已增加固定字段的发布验证报告模板，未取得真机、Windows 主机或签名凭据的项目默认标记为“等待外部验收”。
- 增加生产 TLS 静态回归测试，限制 `InsecureSkipVerify` 只能出现在统一验证连接工厂并绑定 `VerifyConnection`；测试夹具仍可在 `_test.go` 中使用自签名配置。

## 验证结果

- `go test ./... -count=2 -shuffle=on -timeout=180s` 已在最终 TLS 身份和发送恢复范围实现后通过。
- `go test -race ./internal/chat/... -timeout=180s` 已在最终实现后通过，并覆盖多 slot 共享发送恢复范围锁。
- `npm run build` 通过。
- `./gradlew testDebugUnitTest` 和 `./gradlew assembleDebug` 在最终 Android 身份门禁实现后通过；仅有既存 API 弃用警告。
- `./gradlew -PvalidationBuild=true assembleDebugAndroidTest` 通过，隔离 validation applicationId 的 instrumentation APK 已生成；当前 ADB 无连接设备，尚未执行真机测试。
- 本机 IPv4 loopback 已在最终代码上完成 100 MiB TLS/TCP-v3 传输 5 次，SHA-256 全部通过，中位吞吐约 84.86 MiB/s；报告同时记录 completed state、session/generation、checkpoint、durable/final bytes 和 `.part` 清理结果。该结果只属于本机自动化证据，不替代真实双机/三端验收。
- `git diff --check` 通过。

## 尚未自动化的发布门槛

- 100 MiB/1 GiB/10 GiB 三端真实互传、进程重启、Android 锁屏/强杀、网络切换和防火墙隔离。
- 真实进程重启、最终 ACK 丢失、源文件变化、槽位断开、取消/checkpoint 并发与 24 小时 soak 的完整组合矩阵。
- AES-256-GCM 旁表实现、系统密钥库接入、Argon2id 导出、迁移/备份回滚，以及 macOS/Windows 签名安装验证。
- QUIC 生产适配和重复 A/B 性能报告；TLS/TCP 仍是默认实现。
- Android 真机 SAF、锁屏/强杀/网络切换验证完成前保留 `MANAGE_EXTERNAL_STORAGE`，不得宣称权限收敛完成。
