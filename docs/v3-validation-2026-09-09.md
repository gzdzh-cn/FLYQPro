# v3 收尾实施与验证记录（2026-09-09）

状态：本轮代码与自动化回归已落地，整个重构尚未达到发布验收条件。TLS 1.3/TCP 继续作为默认；没有将 QUIC 设置为默认。

## 本轮实际完成

- 修复 Android 帧类型解码重复消费字节、Go ACK 负载长度错误。两端使用相同的 101 字节固定头和同一个 golden packet 回归样本。
- ACK 校验 transfer、session、generation、stream、sequence、offset；EndFile 等待二进制完成确认。空文件和所有 ranges 已完成的任务也执行最终握手。
- 桌面端附件与共享盘发送使用同一个持久 PeerPool/worker。新增实际 TLS 1.3 十文件复用、稀疏恢复、全量恢复、取消、共享预览范围和提交失败测试。
- 修复单 slot 替换错误递增整个 session generation，以及地址竞速取消不生效、落败连接泄漏的问题。证书校验在选中候选之前完成。
- 新发送入口不再声明旧窗口/并行子连接能力；网络控制分发拒绝旧文件数据帧。删除不可达的旧发送循环、共享盘 base64 接收分支、Android writeChunked 和另一套未使用的 QUIC 帧格式。历史 Go helper 和对应历史格式测试仍保留，但生产控制入口禁止使用。
- 共享盘普通下载和预览迁移到 v3 二进制接收器，修复反向数据连接抢先于接收状态注册的竞态。范围预览逐块校验并流式交付；完整文件接收执行整文件 SHA-256。
- 接收 chunk 写入后，以有版本边界的合并 checkpoint 完成文件同步和断点元数据同步，之后才 ACK。网络暂停不再截断稀疏范围；最终提交失败不再被误报成功，保留 .part。跨文件系统复制采用临时文件、校验、同步和 rename。
- Android 文件夹任务进入 SQLite 持久化层，稳定保存文件夹与子任务 ID、子文件进度、错误、暂停和取消状态。清单与子文件共享控制连接；数据连接使用 peer 池，两个文件 worker 调度，每个文件夹子文件最多四个 slot。
- Android 真正拆分新文件的单一缺失范围，多 worker 可以同时工作；单 slot IO 故障重建连接并仅重试未确认范围。恢复不再把发送端旧范围与接收端实际范围合并。
- Android 接收断点保存到数据库；普通文件从原文件读取，消除发送前多一次完整复制，并检查源文件变化。文件夹恢复保留相对路径和子任务身份。
- 修复 GoFrame 默认连接缓存导致关闭后仍使用旧数据库路径的问题，新增重开数据库隔离测试；修复 HTTP 首块测试中的非阻塞断言竞态。

## 已执行的自动化验证

| 验证 | 结果 |
| --- | --- |
| Go 全量 `go test ./... -count=2 -shuffle=on -timeout=180s` | 两轮通过 |
| Go `go test -race ./internal/chat -run '^TestV3' -count=1` | 通过 |
| Android 所有模块 `./gradlew testDebugUnitTest --offline` | 通过：app 19 项，core/data 5 项 |
| Android 普通 debug APK 构建 | 通过 |
| Android 独立包名 validation APK 与 instrumentation 构建 | 通过 |
| macOS arm64、Windows amd64 编译 | 通过；最终生产构建另见交付产物 |
| 两个仓库 `git diff --check` | 通过 |

## 本机性能诊断（不是三端性能验收）

同一本机回环 TLS 用例、100 MiB 数据、4 个 slot：

| 配置 | 数据阶段耗时 | 数据阶段吞吐 | SHA-256 |
| --- | --- | --- | --- |
| 每 chunk 多次单独同步 | 3.239642375 s | 30.87 MiB/s | 一致 |
| 有版本边界的合并 checkpoint | 1.257877666 s | 79.50 MiB/s | 一致 |

这些是单次诊断结果，计时涵盖数据发送及接收最终确认，不涵盖产品发现、用户确认、文件夹扫描、预先整文件 hash。没有用它们声称达到千兆 80%、百兆 85%、WiFi 70% 或相对旧版本减少 30 秒/15%。没有进行真实 TCP/QUIC A/B。

## 真机阻塞

ADB 检测到已授权 OPPO PCDM10。独立验证包 `com.dzh.flyqpro.android.validation` 的安装长时间等待后返回 `Failure [-99]`；设备仍在线，验证包未成功安装。已请求检查手机端 USB 安装确认/拦截。本次没有执行成功的 instrumentation，也没有把编译或 JVM 单元测试当作真机互通结果。

验证 runner 可通过 `-PvalidationBuild=true` 构建，只允许操作独立验证包的数据。包含设备上帧编解码、四段乱序文件写入与 SHA-256、SQLite 重开后恢复文件夹和 ranges、十次任务复用 socket 对象检查。它不是 macOS/Android 网络传输测试，也不是进程被杀/锁屏验收。

## 仍未完成（必须继续追踪）

1. 三端真实 100 MiB、1 GiB、10 GiB 的双向文件/文件夹/共享盘矩阵，以及 Windows 安装运行、Android 锁屏/杀进程/服务重启、网线拔出、网络切换、防火墙/AP 隔离测试。
2. QUIC 的生产调度器接入和 Android 原生 provider，以及同硬件同文件重复 A/B；当前 QUIC 仍不足以作为可发布的三端数据面。
3. 各平台链路类型/真实速率探测、吞吐与磁盘延迟驱动的动态窗口/slot 调整；Android 数据连接仍需要完整的双栈候选竞速与迁移验证。
4. Android 文件夹当前支持可访问的本地目录；SAF document-tree 目录发送、完整 UI 暂停/恢复入口与后台网络自动恢复策略仍需要端到端完善。
5. 对 ACK 丢失发生在最终提交之后、控制 session 整体断开、取消与 checkpoint 并发等更广泛故障进行测试；24 小时生命周期、不可重试错误分类与无人值守自动恢复仍不能视为全部验收通过。
6. 签名/安装包发布验证及正式性能、故障、QUIC A/B 报告。全部达标前，不标记完整重构完成。
