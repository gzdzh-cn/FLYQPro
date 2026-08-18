#!/usr/bin/env bash
set -euo pipefail

# 将插件 resource 和根目录 plugin.json 一起打包到插件自己的 packed 包。
# plugin.json 只保留 addons/<name>/plugin.json 一份，生产环境从 gres 读取
# 打包后的 addons/<name>/resource/plugin.json。

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

for manifest_path in addons/*/plugin.json; do
  [[ -f "${manifest_path}" ]] || continue

  addon_dir="$(dirname "${manifest_path}")"
  addon_name="$(basename "${addon_dir}")"
  resource_dir="${addon_dir}/resource"
  packed_dir="${addon_dir}/packed"
  packed_file="${packed_dir}/packed.go"

  [[ -d "${resource_dir}" ]] || {
    echo "插件 ${addon_name} 缺少 resource 目录" >&2
    exit 1
  }

  mkdir -p "${packed_dir}"
  # packed.go 是生成文件，允许脚本重复执行而不进入交互覆盖确认。
  rm -f "${packed_file}"
  echo "打包插件资源: ${addon_name}"
  gf pack "${resource_dir},${manifest_path}" "${packed_file}" \
    -n packed \
    -p "${resource_dir}"
done

echo "插件资源打包完成"
