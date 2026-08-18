# DMG 打包使用指南

本项目支持为 macOS 创建 DMG 磁盘镜像文件，方便应用程序的分发和安装。

## 快速开始

### 1. 创建默认架构 DMG
```bash
# 这将创建一个默认架构的 DMG（推荐用于一般分发）
wails3 task dmg
```

### 2. 创建通用二进制 DMG
```bash
# 这将创建一个包含 arm64 和 amd64 架构的通用 DMG
wails3 task dmg:universal
```

### 3. 创建特定架构 DMG
```bash
# 仅创建 AMD64 架构的 DMG
wails3 task dmg:amd64
```

### 4. 自定义版本号
```bash
# 指定版本号创建 DMG
wails3 task dmg:universal VERSION=v2.1.0
```

## 任务说明

| 任务 | 描述 | 输出文件 |
|------|------|----------|
| `dmg` | 创建默认架构 DMG | `dzh3159-{version}.dmg` |
| `dmg:universal` | 创建通用二进制 DMG (arm64 + amd64) | `dzh3159-{version}-universal.dmg` |
| `dmg:amd64` | 创建 AMD64 架构 DMG | `dzh3159-{version}-amd64.dmg` |

## 输出位置

所有 DMG 文件都会生成在 `bin/` 目录中。

## 使用流程

1. **构建应用程序**
   ```bash
   # 确保前端已构建
   wails3 task build:frontend
   ```

2. **创建 DMG**
   ```bash
   # 创建默认架构 DMG
   wails3 task dmg
   
   # 或创建通用二进制 DMG
   wails3 task dmg:universal
   ```

3. **分发 DMG**
   - DMG 文件可以直接分发给 macOS 用户
   - 用户双击 DMG 文件后，将应用程序拖拽到 Applications 文件夹即可安装

## 注意事项

- 确保在 macOS 系统上运行 DMG 创建任务
- DMG 文件已经过压缩，文件大小较小
- 生成的 DMG 包含应用程序和 Applications 文件夹快捷方式
- 支持自定义版本号，便于版本管理
- 不同架构的 DMG 文件大小不同：
  - 默认版本：约 25MB
  - AMD64 版本：约 35MB  
  - 通用版本：约 66MB

## 故障排除

如果遇到问题，请检查：
1. 是否在 macOS 系统上运行
2. 是否已经成功构建了应用程序
3. 是否有足够的磁盘空间
4. 脚本权限是否正确 (`chmod +x build/darwin/create_dmg.sh`) 