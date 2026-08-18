# macOS DMG 打包说明

本目录包含了为 macOS 创建 DMG 磁盘镜像的相关任务和脚本。

## 可用的 DMG 打包任务

### 1. 通用二进制 DMG (推荐)
```bash
# 创建通用二进制 (arm64 + amd64) 的 DMG
wails3 task darwin:dmg:universal

# 或者从根目录运行
wails3 task dmg:universal
```

### 2. AMD64 架构 DMG
```bash
# 创建仅 AMD64 架构的 DMG
wails3 task darwin:dmg:amd64

# 或者从根目录运行
wails3 task dmg:amd64
```

### 3. 默认 DMG (基于当前架构)
```bash
# 创建默认 DMG
wails3 task darwin:dmg

# 或者从根目录运行
wails3 task dmg
```

## 自定义版本号

你可以通过设置 `VERSION` 变量来自定义 DMG 文件名中的版本号：

```bash
wails3 task darwin:dmg:universal VERSION=v2.1.0
```

## DMG 内容

生成的 DMG 文件包含：
- 应用程序 (.app 文件)
- Applications 文件夹的快捷方式，方便用户拖拽安装

## 输出位置

DMG 文件将生成在 `bin/` 目录中，文件名格式为：
- `dzh3159-{version}-universal.dmg` (通用二进制)
- `dzh3159-{version}-amd64.dmg` (AMD64 架构)

## 手动创建 DMG

如果需要手动创建 DMG，可以使用提供的脚本：

```bash
cd build/darwin
./create_dmg.sh dzh3159 1.0.0 universal
```

## 注意事项

1. 确保在创建 DMG 之前已经运行了相应的打包任务
2. DMG 创建需要 macOS 系统
3. 生成的 DMG 文件已经过压缩，可以直接分发给用户
4. 用户双击 DMG 文件后，可以将应用程序拖拽到 Applications 文件夹进行安装 