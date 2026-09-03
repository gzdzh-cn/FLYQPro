# 飞秋 Pro v0.1.1

## 版本说明

`v0.1.1` 相比 `v0.1.0` 主要改进如下：

- 完善共享盘协议和共享文件夹浏览能力。
- 支持共享文件列表、缩略图、图片/视频/PDF 预览、下载和另存为。
- 优化共享盘下载任务，支持暂停、继续和取消。
- 优化共享盘设置、目录统计和多共享文件夹管理。
- 优化发现、好友列表、好友申请和好友删除后的状态恢复。
- 修复消息重复回传、未读角标、右键菜单和图片预览相关问题。
- 优化桌面端菜单切换和页面缓存，减少页面切换延迟。
- 增加 Windows MSI 安装包，同时保留 NSIS 安装包。

## 平台版本区别

| 平台 | 适用设备 | 安装包区别 |
| --- | --- | --- |
| macOS ARM64 | Apple Silicon Mac | 原生 ARM64 DMG |
| macOS x86_64 | Intel Mac | 原生 Intel DMG |
| Windows x64 NSIS | Intel/AMD 64 位 Windows | `.exe` 安装包，包含 WebView2 引导器，普通用户优先使用 |
| Windows ARM64 NSIS | ARM64 Windows | `.exe` 安装包，包含 WebView2 引导器 |
| Windows x64 MSI | Intel/AMD 64 位 Windows | `.msi` 安装包，适合系统部署；需要设备已有 WebView2 Runtime |
| Windows ARM64 MSI | ARM64 Windows | `.msi` 安装包，适合系统部署；需要设备已有 WebView2 Runtime |
| Linux | Linux x64 | 根据实际上传内容提供 AppImage、deb、rpm 或 Arch Linux 包 |

Windows 的 NSIS 和 MSI 是两套独立安装包，不需要同时安装。普通用户建议下载 NSIS 版本；需要通过系统软件管理或企业部署时可以选择 MSI 版本。

## 下载地址

以下地址对应 GitHub 仓库 `gzdzh-cn/FLYQPro` 的 `v0.1.1` Release。对应附件上传后即可下载。

### macOS

- [Apple Silicon ARM64 DMG](https://github.com/gzdzh-cn/FLYQPro/releases/download/v0.1.1/FlyQPro-macos-arm64.dmg)
- [Intel x86_64 DMG](https://github.com/gzdzh-cn/FLYQPro/releases/download/v0.1.1/FlyQPro-macos-x86_64.dmg)

### Windows NSIS

- [Windows x64 EXE](https://github.com/gzdzh-cn/FLYQPro/releases/download/v0.1.1/FlyQPro-amd64-installer.exe)
- [Windows ARM64 EXE](https://github.com/gzdzh-cn/FLYQPro/releases/download/v0.1.1/FlyQPro-arm64-installer.exe)

### Windows MSI

- [Windows x64 MSI](https://github.com/gzdzh-cn/FLYQPro/releases/download/v0.1.1/FlyQPro-amd64.msi)
- [Windows ARM64 MSI](https://github.com/gzdzh-cn/FLYQPro/releases/download/v0.1.1/FlyQPro-arm64.msi)

### Linux

Linux 包的文件名以实际 Release 附件为准：[查看全部 Linux 下载文件](https://github.com/gzdzh-cn/FLYQPro/releases/tag/v0.1.1)。

## 安装提示

- macOS 当前版本未完成 Developer ID 签名和 Apple 公证，首次打开可能需要在系统安全设置中确认。
- Windows NSIS 安装包会尝试引导安装 WebView2；MSI 安装包不会安装 WebView2 Runtime。
- 安装前请确认下载的文件与设备 CPU 架构匹配。
- 升级 Android 客户端时，需要使用与原应用相同签名的 APK；Android APK 不在本桌面端 Release 中。

## 校验文件

Release 附件中的 `SHA256SUMS.txt` 用于校验下载完整性：

```bash
shasum -a 256 -c SHA256SUMS.txt
```
