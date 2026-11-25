# 飞秋Pro v0.1.2

本次发布包含 macOS Apple Silicon、macOS Intel 和 Windows AMD64 三个桌面安装包。

## 更新内容

- 增加共享盘和多共享文件夹管理，支持文件浏览、缩略图、图片/视频/PDF 预览、下载及另存为。
- 重构文件传输，默认采用 TLS/TCP v3，完善暂停、继续、取消和断点恢复。
- 优化传输进度、速度及耗时显示，保留取消后的传输信息。
- 完善好友发现、好友申请、删除好友和状态恢复。
- 修复消息重复、未读角标、历史消息右键菜单、亮色主题及引用消息显示问题。
- 优化图片预览、长文本显示、页面切换和 Windows 托盘行为。
- 同步 Windows 程序与安装器版本信息为 0.1.2。

## 下载选择

| 设备 | Release 附件 |
| --- | --- |
| Apple Silicon Mac（M 系列） | `FlyQPro-macos-arm64.dmg` |
| Intel Mac | `FlyQPro-macos-x86_64.dmg` |
| Intel/AMD 64 位 Windows | `FlyQPro-amd64-installer.exe` |

macOS：打开 DMG，将“飞秋Pro.app”拖入“Applications”。Windows：运行 EXE 安装向导；安装器包含 WebView2 引导程序，缺少运行库时需要联网安装。

## 系统与签名说明

- 本次 macOS 包建议使用 macOS 13 或更高版本；未在旧版 macOS 上验证兼容性。
- Windows 安装器要求 Windows 10（Server 2016）或更高版本的 AMD64 系统。
- macOS 包使用本地 ad hoc 签名，未进行 Apple Developer ID 签名与公证；首次运行可能出现系统安全提示。
- Windows 包未进行 Authenticode 签名，可能出现 SmartScreen 提示。

## 本次验证

- Go 全量测试通过。
- 文件传输模块 race 检测通过。
- TLS v3 连续传输 5 个 100 MiB 文件通过，SHA-256 校验与验证报告格式检查通过。
- 前端生产构建通过。
- macOS ARM64 启动检查通过；Intel 版通过本机 Rosetta 启动检查。
- macOS DMG 完整性检查通过。
- Windows AMD64 采用交叉构建；本次未在 Windows 实机上验证安装、运行和卸载。

附件 `SHA256SUMS.txt` 提供 GitHub 下载文件名对应的 SHA-256 校验值；`SHA256SUMS-local-filenames.txt` 供本地中文文件名使用。Android 客户端不包含在本次桌面端发布中。
