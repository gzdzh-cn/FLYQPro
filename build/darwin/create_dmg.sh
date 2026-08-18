#!/bin/bash

# DMG 创建脚本
# 用法: ./create_dmg.sh <app_name> <version> <arch>

APP_NAME=${1:-"dzh3159"}
VERSION=${2:-"1.0.0"}
ARCH=${3:-"universal"}

# 设置路径
BIN_DIR="bin"

# 根据架构设置文件名
if [ "$ARCH" = "default" ]; then
    DMG_NAME="${APP_NAME}-${VERSION}.dmg"
    APP_BUNDLE="${APP_NAME}.app"
elif [ "$ARCH" = "amd64" ]; then
    DMG_NAME="${APP_NAME}-${VERSION}-${ARCH}.dmg"
    APP_BUNDLE="${APP_NAME}-amd64.app"
else
    DMG_NAME="${APP_NAME}-${VERSION}-${ARCH}.dmg"
    APP_BUNDLE="${APP_NAME}.app"
fi

# 检查 .app 是否存在
if [ ! -d "${BIN_DIR}/${APP_BUNDLE}" ]; then
    echo "错误: ${APP_BUNDLE} 不存在于 ${BIN_DIR} 目录中"
    echo "请先运行: wails3 task darwin:package${ARCH:+:$ARCH}"
    exit 1
fi

echo "创建 DMG: ${DMG_NAME}"

# 创建临时目录
TEMP_DIR="${BIN_DIR}/dmg_temp"
rm -rf "${TEMP_DIR}"
mkdir -p "${TEMP_DIR}"

# 复制应用程序
cp -R "${BIN_DIR}/${APP_BUNDLE}" "${TEMP_DIR}/${APP_NAME}.app"

# 创建 Applications 文件夹的符号链接
ln -s /Applications "${TEMP_DIR}/Applications"

# 创建 DMG
echo "正在创建 DMG 文件..."
hdiutil create -volname "${APP_NAME}" -srcfolder "${TEMP_DIR}" -ov -format UDZO "${BIN_DIR}/${DMG_NAME}"

# 清理临时目录
rm -rf "${TEMP_DIR}"

echo "DMG 创建完成: ${BIN_DIR}/${DMG_NAME}"
echo "文件大小: $(du -h "${BIN_DIR}/${DMG_NAME}" | cut -f1)" 